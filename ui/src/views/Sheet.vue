<!--
  Copyright (C) 2026 tebbi
  SPDX-License-Identifier: GPL-3.0-or-later
-->
<template>
  <cv-grid fullWidth>
    <cv-row>
      <cv-column class="page-title">
        <h2>{{ $t("sheet.title") }}</h2>
      </cv-column>
    </cv-row>
    <cv-row>
      <cv-column :md="6" :max="10">
        <cv-tile light class="tile">
          <p class="help">{{ $t("sheet.intro") }}</p>
          <ul class="bullets">
            <li v-for="n in 6" :key="n">{{ $t("sheet.content_" + n) }}</li>
          </ul>
          <NsInlineNotification
            kind="warning"
            :title="$t('sheet.danger_title')"
            :description="$t('sheet.danger')"
            :showCloseButton="false"
          />
          <NsInlineNotification
            kind="info"
            :title="$t('sheet.pdf_title')"
            :description="$t('sheet.pdf')"
            :showCloseButton="false"
          />
          <cv-form @submit.prevent="run('print')">
            <NsTextInput
              :label="$t('sheet.admin_password', { user: userName })"
              type="password"
              v-model="admin_password"
              autocomplete="current-password"
              :helper-text="$t('sheet.admin_password_help')"
              :invalid-message="error.admin_password"
              :disabled="loading"
              ref="admin_password"
            />
            <div class="buttons">
              <NsButton
                kind="primary"
                :icon="Printer20"
                :loading="loading && mode === 'print'"
                :disabled="loading || !admin_password"
                >{{ $t("sheet.print") }}</NsButton
              >
              <NsButton
                kind="secondary"
                :icon="Download20"
                :loading="loading && mode === 'key'"
                :disabled="loading || !admin_password"
                @click.prevent="run('key')"
                >{{ $t("sheet.download_key") }}</NsButton
              >
            </div>
          </cv-form>
          <NsInlineNotification
            v-if="error.sheet"
            kind="error"
            :title="$t('sheet.title')"
            :description="error.sheet"
            :showCloseButton="false"
          />
          <NsInlineNotification
            v-if="done"
            kind="success"
            :title="$t('sheet.done_title')"
            :description="$t('sheet.done_' + done)"
            :showCloseButton="false"
          />
        </cv-tile>
      </cv-column>
    </cv-row>
  </cv-grid>
</template>

<script>
import { mapState } from "vuex";
import Printer20 from "@carbon/icons-vue/es/printer/20";
import Download20 from "@carbon/icons-vue/es/download/20";
import {
  QueryParamService,
  UtilService,
  PageTitleService,
} from "@nethserver/ns8-ui-lib";
import moduleTask from "@/mixins/moduleTask";

// Print layout of the emergency sheet: one A4 page, black on white, large
// monospace characters in boxes so that 0/O and 1/l/I are typed correctly.
const SHEET_CSS = `
@page { size: A4; margin: 8mm 10mm; }
* { box-sizing: border-box; }
body { margin: 0; font-family: "DejaVu Sans", Arial, Helvetica, sans-serif; font-size: 8.2pt; color: #111; }
.page { width: 100%; }
.head { display: flex; justify-content: space-between; align-items: flex-start; border-bottom: 2.5pt solid #111; padding-bottom: 1.5mm; margin-bottom: 2.5mm; }
.title { font-size: 15pt; font-weight: 700; }
.sub { font-size: 8pt; color: #444; margin-top: 1mm; }
.stamp { white-space: pre-line; border: 2pt solid #c00; color: #c00; font-weight: 700; font-size: 10pt; padding: 1.5mm 3mm; letter-spacing: 1px; text-align: center; max-width: 60mm; }
.box { border: 1pt solid #111; border-radius: 2mm; padding: 2mm 3mm 2.2mm; margin-bottom: 2.2mm; break-inside: avoid; }
.box-h { font-size: 10pt; font-weight: 700; margin-bottom: 1.2mm; display: flex; align-items: center; gap: 2mm; }
.num { display: inline-flex; width: 5.5mm; height: 5.5mm; border-radius: 50%; background: #111; color: #fff; font-size: 8.5pt; align-items: center; justify-content: center; }
.label { font-size: 7pt; text-transform: uppercase; letter-spacing: .5px; color: #444; margin: 1.2mm 0 .6mm; }
.chars { display: flex; flex-wrap: wrap; gap: .6mm; }
.c { display: inline-flex; flex-direction: column; align-items: center; width: 4.8mm; }
.ch { display: block; width: 4.8mm; height: 6mm; line-height: 6mm; text-align: center; border: .6pt solid #777; border-radius: .8mm; font-family: "DejaVu Sans Mono", "Courier New", monospace; font-size: 11pt; font-weight: 700; }
.pos { font-size: 5pt; color: #666; height: 2mm; }
.hex { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1mm 2.5mm; margin-bottom: 1mm; }
.g { display: flex; align-items: baseline; gap: 1.5mm; border: .6pt solid #777; border-radius: .8mm; padding: .5mm 1.5mm; }
.gi { font-size: 6.5pt; color: #666; }
.gv { font-family: "DejaVu Sans Mono", "Courier New", monospace; font-size: 12pt; font-weight: 700; letter-spacing: 1px; }
.xml { margin: .8mm 0; padding: 1mm 2mm; background: #f2f2f2; border-radius: 1mm; font-family: "DejaVu Sans Mono", "Courier New", monospace; font-size: 6.5pt; white-space: pre; color: #333; }
.note { font-size: 7pt; color: #333; margin: .6mm 0; }
.none { color: #666; font-style: italic; }
.cols { display: grid; grid-template-columns: 1fr 1fr; gap: 2.5mm; }
.cols .box { margin-bottom: 2.2mm; }
.kv { width: 100%; border-collapse: collapse; }
.kv td { padding: .5mm 0; vertical-align: top; border-bottom: .4pt solid #ccc; }
.kv .k { width: 30%; font-size: 7.5pt; color: #444; padding-right: 2mm; }
.kv .mono { font-family: "DejaVu Sans Mono", "Courier New", monospace; font-size: 7.5pt; word-break: break-all; }
.lines { display: grid; grid-template-columns: 1fr; gap: 0; }
.line { display: flex; align-items: flex-end; gap: 2mm; height: 6.8mm; }
.ll { width: 62mm; white-space: nowrap; font-size: 7.5pt; color: #333; padding-bottom: .8mm; }
.lw { flex: 1; border-bottom: .7pt solid #111; height: 100%; }
.steps { margin: 0; padding-left: 5mm; }
.steps li { margin-bottom: .4mm; }
.foot { text-align: center; font-size: 8pt; font-weight: 700; color: #c00; margin-top: 1mm; }
`;

const b64 = (s) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

export default {
  name: "Sheet",
  mixins: [moduleTask, UtilService, QueryParamService, PageTitleService],
  pageTitle() {
    return this.$t("sheet.title") + " - " + this.appName;
  },
  data() {
    return {
      q: { page: "sheet" },
      urlCheckInterval: null,
      Printer20,
      Download20,
      admin_password: "",
      loading: false,
      mode: "",
      done: "",
      error: { admin_password: "", sheet: "" },
    };
  },
  computed: {
    ...mapState(["instanceName", "core", "appName"]),
    userName() {
      const u = this.core && this.core.$store && this.core.$store.state;
      return (u && u.loggedUser) || "admin";
    },
  },
  beforeRouteEnter(to, from, next) {
    next((vm) => {
      vm.watchQueryData(vm);
      vm.urlCheckInterval = vm.initUrlBindingForApp(vm, vm.q.page);
    });
  },
  beforeRouteLeave(to, from, next) {
    clearInterval(this.urlCheckInterval);
    this.admin_password = "";
    next();
  },
  methods: {
    async open(sealed, password) {
      // PBKDF2-SHA256 + AES-256-GCM, the counterpart of "pbexport seal"
      const enc = new TextEncoder();
      const base = await window.crypto.subtle.importKey(
        "raw",
        enc.encode(password),
        "PBKDF2",
        false,
        ["deriveKey"]
      );
      const key = await window.crypto.subtle.deriveKey(
        {
          name: "PBKDF2",
          hash: "SHA-256",
          salt: b64(sealed.salt),
          iterations: sealed.iterations,
        },
        base,
        { name: "AES-GCM", length: 256 },
        false,
        ["decrypt"]
      );
      const plain = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: b64(sealed.nonce) },
        key,
        b64(sealed.ciphertext)
      );
      return JSON.parse(new TextDecoder().decode(plain));
    },
    async run(mode) {
      this.error.admin_password = "";
      this.error.sheet = "";
      this.done = "";
      this.mode = mode;
      this.loading = true;
      const password = this.admin_password;
      this.admin_password = "";
      let sheet = null;
      try {
        const res = await this.runModuleTask("get-emergency-sheet", {
          admin_password: password,
        });
        sheet = await this.open(res.sealed, password);
        if (mode === "key") {
          this.downloadKey(sheet);
        } else {
          this.printSheet(sheet, res.user);
        }
        this.done = mode;
      } catch (e) {
        if (e.validation) {
          const v = e.validation[0] || {};
          this.error.admin_password = this.$t(
            "sheet.error_" + (v.error || "admin_password_invalid")
          );
        } else {
          this.error.sheet = this.$t("error.generic_error");
        }
      } finally {
        sheet = null;
        this.loading = false;
      }
    },
    downloadKey(sheet) {
      if (!sheet.private_key) {
        throw new Error("no key");
      }
      const blob = new Blob([sheet.private_key], {
        type: "application/pgp-keys",
      });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      const who = (sheet.passbolt_user || "export").replace(
        /[^A-Za-z0-9._-]/g,
        "_"
      );
      a.download = "passbolt-recovery-kit-" + who + ".asc";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(a.href);
    },
    visibleSpaces(s) {
      return (s || "").replace(/ /g, "␣");
    },
    printSheet(sheet, user) {
      // The page is built with DOM methods only (textContent), never from
      // HTML strings, and removed right after printing.
      const frame = document.createElement("iframe");
      frame.style.position = "fixed";
      frame.style.width = "0";
      frame.style.height = "0";
      frame.style.border = "0";
      document.body.appendChild(frame);
      const doc = frame.contentDocument;
      doc.open();
      doc.write("<!DOCTYPE html><html><head></head><body></body></html>");
      doc.close();
      const t = (k, p) => this.$t("sheet.p_" + k, p);
      const el = (tag, parent, text, cls) => {
        const e = doc.createElement(tag);
        if (text !== undefined && text !== null) e.textContent = text;
        if (cls) e.className = cls;
        parent.appendChild(e);
        return e;
      };
      el("style", doc.head).textContent = SHEET_CSS;
      const page = el("div", doc.body, undefined, "page");

      // header
      const head = el("div", page, undefined, "head");
      const left = el("div", head);
      el("div", left, t("heading"), "title");
      el(
        "div",
        left,
        t("created", {
          date: new Date(sheet.created * 1000).toLocaleString(),
          user,
          module: sheet.module_id,
        }),
        "sub"
      );
      el("div", head, t("confidential_short"), "stamp");

      const box = (n, title, cls) => {
        const b = el("div", page, undefined, "box " + (cls || ""));
        const h = el("div", b, undefined, "box-h");
        el("span", h, String(n), "num");
        el("span", h, title);
        return b;
      };
      // one character per cell, numbered every 5 positions
      const chars = (parent, label, value) => {
        el("div", parent, label, "label");
        const row = el("div", parent, undefined, "chars");
        [...(value || "")].forEach((c, i) => {
          const cell = el("span", row, undefined, "c");
          el("span", cell, c === " " ? "␣" : c, "ch");
          el("span", cell, (i + 1) % 5 === 0 ? String(i + 1) : "", "pos");
        });
        if (!value) el("span", row, "-", "none");
      };
      const kv = (parent, rows) => {
        const tb = el("table", parent, undefined, "kv");
        for (const [k, v, mono] of rows) {
          const tr = el("tr", tb);
          el("td", tr, k, "k");
          el("td", tr, v || "-", mono ? "v mono" : "v");
        }
      };

      // 1: open the file
      const b1 = box(1, t("box_open"));
      chars(b1, t("kdbx_password"), sheet.kdbx_password);
      el("div", b1, t("keyfile"), "label");
      if (sheet.keyfile_hex) {
        const groups = sheet.keyfile_hex.match(/.{8}/g);
        const grid = el("div", b1, undefined, "hex");
        groups.forEach((g, i) => {
          const cell = el("span", grid, undefined, "g");
          el("span", cell, String(i + 1), "gi");
          el("span", cell, g, "gv");
        });
        el("div", b1, t("keyfile_how_short"), "note");
        el(
          "pre",
          b1,
          '<?xml version="1.0" encoding="utf-8"?>\n<KeyFile><Meta><Version>2.0</Version></Meta><Key>\n' +
            '  <Data Hash="' +
            sheet.keyfile_hash +
            '">' +
            groups.slice(0, 4).join(" ") +
            "\n        " +
            groups.slice(4).join(" ") +
            "</Data>\n</Key></KeyFile>",
          "xml"
        );
        el("div", b1, t("keyfile_hex_short"), "note");
      } else {
        el("div", b1, t("no_keyfile"), "none");
      }

      const cols = el("div", page, undefined, "cols");
      // 2: where the file is
      const b2 = box(2, t("box_where"));
      cols.appendChild(b2);
      const where = sheet.target_type
        ? this.$t("target." + sheet.target_type) +
          " · " +
          (sheet.target_path || "/")
        : "-";
      kv(b2, [
        [t("where"), where],
        [t("file"), sheet.last_file, true],
        [t("sha256"), sheet.last_sha256, true],
      ]);
      // 3: passbolt account
      const b3 = box(3, t("account_heading_short"));
      cols.appendChild(b3);
      kv(b3, [
        [t("passbolt_url"), sheet.passbolt_url, true],
        [t("passbolt_user"), sheet.passbolt_user, true],
        [t("private_key"), t("private_key_short")],
      ]);
      chars(b3, t("passphrase"), sheet.passphrase);
      chars(b3, t("totp"), sheet.totp_secret);

      // 4: to fill in
      const b4 = box(4, t("blank_heading"));
      const lines = el("div", b4, undefined, "lines");
      for (const k of [
        "target_user",
        "target_password",
        "target_totp",
        "target_codes",
        "target_token",
        "ns8_backup_key",
        "ns8_backup_where",
      ]) {
        const l = el("div", lines, undefined, "line");
        el("span", l, t(k), "ll");
        el("span", l, "", "lw");
      }

      // 5: steps
      const b5 = box(5, t("steps_heading"));
      const ol = el("ol", b5, undefined, "steps");
      for (let n = 1; n <= 4; n++) el("li", ol, t("step_" + n));

      el("div", page, t("footer"), "foot");

      const cleanup = () => {
        if (frame.parentNode) frame.parentNode.removeChild(frame);
      };
      frame.contentWindow.addEventListener("afterprint", () =>
        setTimeout(cleanup, 500)
      );
      setTimeout(cleanup, 120000);
      frame.contentWindow.focus();
      frame.contentWindow.print();
    },
  },
};
</script>

<style scoped lang="scss">
@import "../styles/carbon-utils";
.tile {
  margin-bottom: $spacing-06;
}
.help {
  margin-bottom: $spacing-05;
}
.bullets {
  list-style: disc;
  margin: 0 0 $spacing-05 $spacing-06;
  li {
    margin-bottom: $spacing-03;
  }
}
.buttons {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-05;
  margin: $spacing-05 0;
}
</style>
