//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/text/encoding/charmap"
)

// The emergency sheet as a one-page PDF, encrypted with AES-256 (PDF 2.0,
// revision 6). The open password is the administrator's password, the
// owner password is random, so the file can be printed but not changed.
// The page uses the standard PDF fonts (no embedding, WinAnsi): enough for
// English, German, Italian and French; a character outside WinAnsi in a
// secret is printed as its code point.

type sheetData struct {
	Created      string `json:"created"`
	ModuleID     string `json:"module_id"`
	User         string `json:"user"`
	PassboltURL  string `json:"passbolt_url"`
	PassboltUser string `json:"passbolt_user"`
	Passphrase   string `json:"passphrase"`
	TOTPSecret   string `json:"totp_secret"`
	KdbxPassword string `json:"kdbx_password"`
	KeyfileHex   string `json:"keyfile_hex"`
	KeyfileHash  string `json:"keyfile_hash"`
	Where        string `json:"where"`
	LastFile     string `json:"last_file"`
	LastSHA256   string `json:"last_sha256"`
}

type pdfRequest struct {
	Password string            `json:"password"`
	Sheet    sheetData         `json:"sheet"`
	Labels   map[string]string `json:"labels"`
}

const (
	pageW   = 595.28
	pageH   = 841.89
	marginX = 30.0
	topY    = 812.0
)

var cp1252 = charmap.Windows1252.NewEncoder()

// winAnsi converts to the WinAnsi bytes of the standard fonts; runes
// without a WinAnsi glyph become "?".
func winAnsi(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\u2423' { // the "visible space" of the labels
			b.WriteByte('_')
			continue
		}
		e, err := cp1252.String(string(r))
		if err != nil || r == utf8.RuneError {
			b.WriteByte('?')
			continue
		}
		b.WriteString(e)
	}
	return b.String()
}

func pdfString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, "\r", " ", "\n", " ")
	return "(" + r.Replace(winAnsi(s)) + ")"
}

type canvas struct {
	b bytes.Buffer
}

var fontRes = map[string]string{"Helvetica": "F1", "Helvetica-Bold": "F2", "Courier": "F3", "Courier-Bold": "F4"}

func textWidth(s, fontName string, size float64) float64 {
	w, err := font.TextWidthFloat(context.Background(), winAnsi(s), fontName, size)
	if err != nil {
		return float64(len(s)) * size * 0.55
	}
	return w
}

func (c *canvas) rgb(r, g, b float64) {
	fmt.Fprintf(&c.b, "%.3f %.3f %.3f rg %.3f %.3f %.3f RG\n", r, g, b, r, g, b)
}

func (c *canvas) text(x, y float64, fontName string, size float64, s string) {
	fmt.Fprintf(&c.b, "BT /%s %.2f Tf %.2f %.2f Td %s Tj ET\n", fontRes[fontName], size, x, y, pdfString(s))
}

func (c *canvas) centered(cx, y float64, fontName string, size float64, s string) {
	c.text(cx-textWidth(s, fontName, size)/2, y, fontName, size, s)
}

func (c *canvas) rect(x, y, w, h, lw float64, fill bool) {
	op := "S"
	if fill {
		op = "f"
	}
	fmt.Fprintf(&c.b, "%.2f w %.2f %.2f %.2f %.2f re %s\n", lw, x, y, w, h, op)
}

func (c *canvas) line(x1, y1, x2, y2, lw float64) {
	fmt.Fprintf(&c.b, "%.2f w %.2f %.2f m %.2f %.2f l S\n", lw, x1, y1, x2, y2)
}

// wrap splits s into lines no wider than width.
func wrap(s, fontName string, size, width float64) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := strings.TrimSpace(line + " " + word)
			if line != "" && textWidth(try, fontName, size) > width {
				lines = append(lines, line)
				line = word
			} else {
				line = try
			}
			// a single word longer than the width (hashes, keys): split it
			for textWidth(line, fontName, size) > width && len(line) > 1 {
				cut := len(line) - 1
				for cut > 1 && textWidth(line[:cut], fontName, size) > width {
					cut--
				}
				lines = append(lines, line[:cut])
				line = line[cut:]
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// paragraph draws wrapped text from y downwards and returns the new y.
func (c *canvas) paragraph(x, y, width float64, fontName string, size float64, s string) float64 {
	for _, l := range wrap(s, fontName, size, width) {
		c.text(x, y-size, fontName, size, l)
		y -= size * 1.3
	}
	return y
}

type sheetPDF struct {
	c      canvas
	labels map[string]string
	s      sheetData
}

func (p *sheetPDF) l(key string) string {
	if v, ok := p.labels[key]; ok {
		return v
	}
	return key
}

// boxHeader draws the numbered title of a box and returns the y below it.
func (p *sheetPDF) boxHeader(x, y float64, n int, title string) float64 {
	p.c.rgb(0, 0, 0)
	p.c.rect(x+8, y-20, 13, 13, 0, true)
	p.c.rgb(1, 1, 1)
	p.c.centered(x+14.5, y-17, "Helvetica-Bold", 8.5, fmt.Sprint(n))
	p.c.rgb(0, 0, 0)
	p.c.text(x+27, y-17.5, "Helvetica-Bold", 10.5, title)
	return y - 26
}

func (p *sheetPDF) label(x, y float64, s string) float64 {
	p.c.rgb(0.27, 0.27, 0.27)
	p.c.text(x, y-7, "Helvetica", 6.8, strings.ToUpper(s))
	p.c.rgb(0, 0, 0)
	return y - 10
}

// chars draws a secret one character per cell, numbered every five.
func (p *sheetPDF) chars(x, y, width float64, value string) float64 {
	const cw, ch, gap = 13.0, 17.0, 1.8
	if value == "" {
		p.c.text(x, y-10, "Helvetica-Oblique", 8, "-")
		return y - 14
	}
	cx, cy := x, y
	for i, r := range []rune(value) {
		if cx+cw > x+width {
			cx = x
			cy -= ch + 8
		}
		p.c.rgb(0.45, 0.45, 0.45)
		p.c.rect(cx, cy-ch, cw, ch, 0.6, false)
		p.c.rgb(0, 0, 0)
		switch {
		case r == ' ':
			// visible space: a small open bracket
			p.c.line(cx+3, cy-12, cx+3, cy-14, 0.8)
			p.c.line(cx+3, cy-14, cx+cw-3, cy-14, 0.8)
			p.c.line(cx+cw-3, cy-14, cx+cw-3, cy-12, 0.8)
		case winAnsi(string(r)) == "?" && r != '?':
			p.c.centered(cx+cw/2, cy-11, "Helvetica", 4.5, fmt.Sprintf("U+%04X", r))
		default:
			p.c.centered(cx+cw/2, cy-12.5, "Courier-Bold", 12, string(r))
		}
		if (i+1)%5 == 0 {
			p.c.rgb(0.4, 0.4, 0.4)
			p.c.centered(cx+cw/2, cy-ch-6, "Helvetica", 5, fmt.Sprint(i+1))
			p.c.rgb(0, 0, 0)
		}
		cx += cw + gap
	}
	return cy - ch - 9
}

func (p *sheetPDF) kv(x, y, width float64, rows [][3]string) float64 {
	const lw = 0.3
	keyW := width * 0.3
	for _, r := range rows {
		valueFont := "Helvetica"
		if r[2] == "mono" {
			valueFont = "Courier"
		}
		value := r[1]
		if value == "" {
			value = "-"
		}
		lines := wrap(value, valueFont, 7.2, width-keyW-4)
		p.c.rgb(0.27, 0.27, 0.27)
		p.c.text(x, y-8, "Helvetica", 7, r[0])
		p.c.rgb(0, 0, 0)
		ly := y
		for _, l := range lines {
			p.c.text(x+keyW, ly-8, valueFont, 7.2, l)
			ly -= 9
		}
		y = ly - 2.5
		p.c.rgb(0.8, 0.8, 0.8)
		p.c.line(x, y+1, x+width, y+1, lw)
		p.c.rgb(0, 0, 0)
	}
	return y - 1
}

func (p *sheetPDF) frame(x, yTop, w, yBottom float64) {
	p.c.rgb(0, 0, 0)
	p.c.rect(x, yBottom, w, yTop-yBottom, 0.9, false)
}

func (p *sheetPDF) render() []byte {
	s := p.s
	c := &p.c
	contentW := pageW - 2*marginX
	y := topY

	// header
	c.rgb(0, 0, 0)
	c.text(marginX, y-16, "Helvetica-Bold", 16, p.l("heading"))
	created := strings.NewReplacer("{date}", s.Created, "{user}", s.User, "{module}", s.ModuleID).Replace(p.l("created"))
	c.rgb(0.27, 0.27, 0.27)
	c.text(marginX, y-29, "Helvetica", 7.5, created)
	stampW := 165.0
	c.rgb(0.8, 0, 0)
	c.rect(pageW-marginX-stampW, y-34, stampW, 34, 1.8, false)
	stamp := strings.SplitN(p.l("confidential_short"), "\n", 2)
	c.centered(pageW-marginX-stampW/2, y-14, "Helvetica-Bold", 9.5, stamp[0])
	if len(stamp) > 1 {
		c.centered(pageW-marginX-stampW/2, y-27, "Helvetica", 8.5, stamp[1])
	}
	c.rgb(0, 0, 0)
	y -= 40
	c.line(marginX, y, pageW-marginX, y, 2)
	y -= 7

	inner := contentW - 18
	ix := marginX + 9

	// 1: open the KeePass file
	top := y
	y = p.boxHeader(marginX, y, 1, p.l("box_open"))
	y = p.label(ix, y, p.l("kdbx_password"))
	y = p.chars(ix, y, inner, s.KdbxPassword)
	y = p.label(ix, y, p.l("keyfile"))
	if len(s.KeyfileHex) == 64 {
		gw := (inner - 3*7) / 4
		for i := 0; i < 8; i++ {
			gx := ix + float64(i%4)*(gw+7)
			gy := y - float64(i/4)*22
			c.rgb(0.45, 0.45, 0.45)
			c.rect(gx, gy-19, gw, 19, 0.6, false)
			c.rgb(0.4, 0.4, 0.4)
			c.text(gx+4, gy-13, "Helvetica", 6, fmt.Sprint(i+1))
			c.rgb(0, 0, 0)
			c.text(gx+14, gy-14, "Courier-Bold", 12.5, s.KeyfileHex[i*8:(i+1)*8])
		}
		y -= 2*22 + 3
		c.rgb(0.2, 0.2, 0.2)
		y = c.paragraph(ix, y, inner, "Helvetica", 7, p.l("keyfile_how_short"))
		groups := make([]string, 8)
		for i := range groups {
			groups[i] = s.KeyfileHex[i*8 : (i+1)*8]
		}
		xml := []string{
			`<?xml version="1.0" encoding="utf-8"?>`,
			`<KeyFile><Meta><Version>2.0</Version></Meta><Key>`,
			`  <Data Hash="` + s.KeyfileHash + `">` + strings.Join(groups[:4], " "),
			`        ` + strings.Join(groups[4:], " ") + `</Data>`,
			`</Key></KeyFile>`,
		}
		c.rgb(0.94, 0.94, 0.94)
		c.rect(ix, y-float64(len(xml))*8.2-5, inner, float64(len(xml))*8.2+4, 0, true)
		c.rgb(0.2, 0.2, 0.2)
		for _, l := range xml {
			c.text(ix+5, y-8, "Courier", 6.8, l)
			y -= 8.2
		}
		y -= 6
		y = c.paragraph(ix, y, inner, "Helvetica", 7, p.l("keyfile_hex_short"))
		c.rgb(0, 0, 0)
	} else {
		c.text(ix, y-9, "Helvetica-Oblique", 8, p.l("no_keyfile"))
		y -= 13
	}
	y -= 4
	p.frame(marginX, top, contentW, y)
	y -= 7

	// 2 and 3 side by side
	colW := (contentW - 8) / 2
	top = y
	ly := p.boxHeader(marginX, y, 2, p.l("box_where"))
	ly = p.kv(marginX+9, ly, colW-18, [][3]string{
		{p.l("where"), s.Where, ""},
		{p.l("file"), s.LastFile, "mono"},
		{p.l("sha256"), s.LastSHA256, "mono"},
	})
	rx := marginX + colW + 8
	ry := p.boxHeader(rx, y, 3, p.l("account_heading_short"))
	ry = p.kv(rx+9, ry, colW-18, [][3]string{
		{p.l("passbolt_url"), s.PassboltURL, "mono"},
		{p.l("passbolt_user"), s.PassboltUser, "mono"},
		{p.l("private_key"), p.l("private_key_short"), ""},
	})
	ry = p.label(rx+9, ry, p.l("passphrase"))
	ry = p.chars(rx+9, ry, colW-18, s.Passphrase)
	ry = p.label(rx+9, ry, p.l("totp"))
	ry = p.chars(rx+9, ry, colW-18, s.TOTPSecret)
	bottom := min(ly, ry) - 4
	p.frame(marginX, top, colW, bottom)
	p.frame(rx, top, colW, bottom)
	y = bottom - 7

	// 4: blank lines
	top = y
	y = p.boxHeader(marginX, y, 4, p.l("blank_heading"))
	for _, k := range []string{"target_user", "target_password", "target_totp", "target_codes", "target_token",
		"ns8_backup_key", "ns8_backup_where"} {
		c.rgb(0.2, 0.2, 0.2)
		c.text(ix, y-16, "Helvetica", 7.5, p.l(k))
		c.rgb(0, 0, 0)
		c.line(ix+185, y-17, ix+inner, y-17, 0.7)
		y -= 20
	}
	y -= 10
	p.frame(marginX, top, contentW, y)
	y -= 7

	// 5: steps
	top = y
	y = p.boxHeader(marginX, y, 5, p.l("steps_heading"))
	for i := 1; i <= 4; i++ {
		c.text(ix, y-8, "Helvetica", 7.8, fmt.Sprintf("%d.", i))
		y = c.paragraph(ix+12, y, inner-12, "Helvetica", 7.8, p.l(fmt.Sprintf("step_%d", i))) - 1
	}
	y -= 3
	p.frame(marginX, top, contentW, y)
	y -= 14
	c.rgb(0.8, 0, 0)
	c.centered(pageW/2, y, "Helvetica-Bold", 8, p.l("footer"))

	return p.document()
}

// document wraps the content stream into a minimal PDF file.
func (p *sheetPDF) document() []byte {
	content := p.c.b.Bytes()
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Contents 4 0 R /Resources << /Font << "+
			"/F1 5 0 R /F2 6 0 R /F3 7 0 R /F4 8 0 R /F5 9 0 R >> >> >>", pageW, pageH),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content)+1, content),
	}
	for _, f := range []string{"Helvetica", "Helvetica-Bold", "Courier", "Courier-Bold", "Helvetica-Oblique"} {
		objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /"+f+" /Encoding /WinAnsiEncoding >>")
	}
	var out bytes.Buffer
	out.WriteString("%PDF-2.0\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

func init() {
	fontRes["Helvetica-Oblique"] = "F5"
}

// encryptPDF applies AES-256 with the administrator's password as open
// password and a random owner password; printing stays allowed.
func encryptPDF(plain []byte, userPW string) ([]byte, error) {
	api.DisableConfigDir()
	owner := make([]byte, 24)
	if _, err := rand.Read(owner); err != nil {
		return nil, err
	}
	conf := model.NewAESConfiguration(userPW, hex.EncodeToString(owner), 256)
	conf.Permissions = model.PermissionsPrint
	var out bytes.Buffer
	if err := api.Encrypt(context.Background(), bytes.NewReader(plain), &out, conf); err != nil {
		return nil, fmt.Errorf("encrypting the PDF: %w", err)
	}
	return out.Bytes(), nil
}

func buildSheetPDF(r io.Reader) ([]byte, string, error) {
	var req pdfRequest
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err == nil && len(data) > maxConfigSize {
		err = errors.New("the request is too large")
	}
	if err == nil {
		err = json.Unmarshal(data, &req)
	}
	if err == nil && req.Password == "" {
		err = errors.New("the password is required")
	}
	if err != nil {
		return nil, "", err
	}
	if len(req.Labels) > 100 {
		return nil, "", errors.New("too many labels")
	}
	p := &sheetPDF{labels: req.Labels, s: req.Sheet}
	enc, err := encryptPDF(p.render(), req.Password)
	return enc, req.Password, err
}

// runPDF writes the encrypted PDF sealed for the browser (see seal.go):
// Redis only ever sees the PBKDF2/AES-GCM envelope.
func runPDF() {
	pdf, password, err := buildSheetPDF(os.Stdin)
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(sealed{Error: err.Error()})
		os.Exit(1)
	}
	doc, _ := json.Marshal(sealRequest{Password: password, Plaintext: base64.StdEncoding.EncodeToString(pdf)})
	runSealFrom(bytes.NewReader(doc))
}
