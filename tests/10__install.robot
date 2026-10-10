*** Settings ***
Library     SSHLibrary
Resource    api.resource

*** Test Cases ***
Install the module
    IF    '${SCENARIO}' == 'update'
        ${output}  ${rc} =    Execute Command    add-module ${UPDATE_FROM} 1    return_rc=True
    ELSE
        ${output}  ${rc} =    Execute Command    add-module ${IMAGE_URL} 1    return_rc=True
    END
    Should Be Equal As Integers    ${rc}  0
    &{output} =    Evaluate    ${output}
    Set Global Variable    ${module_id}    ${output.module_id}

The module keeps its selfadm role
    The module may still configure itself    ${module_id}

A fresh instance has no timer and nothing to export
    ${cfg} =    Run task    module/${module_id}/get-configuration    {}
    Should Not Be True    ${cfg['target_configured']}
    Should Be Equal    ${cfg['next_run']}    ${NONE}
    Run on node    runagent -m ${module_id} bash -c '! systemctl --user is-enabled passbolt-export.timer'

Invalid settings are refused
    ${cfg} =    Config    passbolt_url=http://passbolt.ci.test
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    passbolt_url_invalid
    ${cfg} =    Config    kdbx_password=short
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    kdbx_password_too_short
    ${cfg} =    Config    kdbx_keyfile_secret=not a key file
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    keyfile_invalid
    ${cfg} =    Config    target_path=../escape
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    target_path_invalid
    ${cfg} =    Config    passbolt_private_key=no key
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    private_key_invalid
    ${cfg} =    Config    webdav_url=http://dav.ci.test/
    Run task expecting a validation error    module/${module_id}/configure-module    ${cfg}    https_required

Configure the module
    ${cfg} =    Config
    Run task    module/${module_id}/configure-module    ${cfg}    decode_json=${FALSE}

Update to the image under test
    Skip If    '${SCENARIO}' != 'update'    scenario is ${SCENARIO}
    Run on node    api-cli run update-module --data '{"force":true,"module_url":"${IMAGE_URL}","instances":["${module_id}"]}'

Configuration reads back without secrets
    ${cfg} =    Run task    module/${module_id}/get-configuration    {}
    Should Be Equal    ${cfg['passbolt_url']}    https://passbolt.ci.test:9
    Should Be Equal    ${cfg['target_type']}    webdav
    Should Be Equal    ${cfg['target_path']}    Passbolt/CI
    Should Be True    ${cfg['private_key_set']}
    Should Be True    ${cfg['passphrase_set']}
    Should Be True    ${cfg['totp_secret_set']}
    Should Be True    ${cfg['kdbx_password_set']}
    Should Be True    ${cfg['keyfile_set']}
    Should Be True    ${cfg['target_configured']}
    Should Not Be Equal    ${cfg['next_run']}    ${NONE}
    ${text} =    Evaluate    json.dumps($cfg)    modules=json
    Should Not Contain    ${text}    ci-passphrase-4711
    Should Not Contain    ${text}    BEGIN PGP

Empty secret fields keep the stored values
    ${cfg} =    Keep Secrets    retention=${7}
    Run task    module/${module_id}/configure-module    ${cfg}    decode_json=${FALSE}
    ${cfg} =    Run task    module/${module_id}/get-configuration    {}
    Should Be Equal As Integers    ${cfg['retention']}    7
    Should Be True    ${cfg['passphrase_set']}
    Should Be True    ${cfg['keyfile_set']}
    Should Be True    ${cfg['target_configured']}

Secrets are stored in passwords.env and rclone.conf only
    Secrets are kept out of the module environment    ${module_id}
    ${mode} =    Run on node    runagent -m ${module_id} bash -c 'stat -c %a "$AGENT_STATE_DIR/rclone/rclone.conf" "$AGENT_STATE_DIR/rclone"'
    Should Be Equal    ${mode.split()}    ${{ ["600", "700"] }}
    # rclone keeps passwords obscured, never as typed
    ${hits} =    Run on node    runagent -m ${module_id} bash -c 'grep -c -F ci-dav-password-42 "$AGENT_STATE_DIR/rclone/rclone.conf" || true'
    Should Be Equal As Integers    ${hits.strip()}    0

The schedule is applied
    Run on node    runagent -m ${module_id} systemctl --user is-enabled passbolt-export.timer
    ${unit} =    Run on node    runagent -m ${module_id} systemctl --user cat passbolt-export.timer
    Should Contain    ${unit}    OnCalendar=*-*-15 04:15:00

A failing connection is reported, not crashed
    ${out} =    Run task    module/${module_id}/check-connection    {}
    Should Not Be True    ${out['ok']}
    Should Not Be Empty    ${out['error']}

A failing export is recorded
    Run task    module/${module_id}/run-export    {}    rc_expected=1    decode_json=${FALSE}
    ${cfg} =    Run task    module/${module_id}/get-configuration    {}
    Should Not Be True    ${cfg['runs'][0]['ok']}
    Should Be Equal    ${cfg['runs'][0]['trigger']}    manual
    Should Be Equal    ${cfg['runs'][0]['mail']}    skipped

No secret leaks into Redis or the journal
    No test secret is visible outside the secrets files    ${module_id}

The emergency sheet needs the password of the administrator
    [Documentation]    A wrong password is refused and, after five tries,
    ...                the sheet is locked. The output never holds a secret.
    FOR    ${i}    IN RANGE    5
        Run task expecting a validation error    module/${module_id}/get-emergency-sheet    {"admin_password":"wrong-${i}"}    admin_password_invalid
    END
    Run task expecting a validation error    module/${module_id}/get-emergency-sheet    {"admin_password":"wrong-again"}    sheet_locked
    Run on node    runagent -m ${module_id} rm -f .sheet-attempts.json
