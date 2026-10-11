*** Settings ***
Library     SSHLibrary
Resource    api.resource

*** Test Cases ***
Back up the module
    ${sums} =    Target login checksum    ${module_id}
    Set Global Variable    ${SUMS_ORIG}    ${sums}
    ${repo}    ${path} =    Back up the module to the cluster repository    ${module_id}
    Set Global Variable    ${BACKUP_REPO}    ${repo}
    Set Global Variable    ${BACKUP_PATH}    ${path}

Stop the original instance
    # Stopped, not removed: on Rocky 9 (systemd 252) a module removed and
    # re-created within seconds gets the same UID back and the user manager
    # is not started again.
    Run on node    runagent -m ${module_id} systemctl --user disable --now passbolt-export.timer

Restore into a new instance
    ${rid} =    Restore the module from the cluster repository    ${BACKUP_REPO}    ${BACKUP_PATH}
    Set Global Variable    ${restored_id}    ${rid}
    Should Not Be Equal    ${restored_id}    ${module_id}

The restored instance has the settings, the target login and the history
    ${cfg} =    Run task    module/${restored_id}/get-configuration    {}
    Should Be Equal    ${cfg['passbolt_url']}    https://passbolt.ci.test:9
    Should Be Equal As Integers    ${cfg['retention']}    7
    Should Be Equal As Integers    ${cfg['schedule_monthday']}    15
    Should Be True    ${cfg['keyfile_enabled']}    the key file option must survive a restore without the key file
    Should Be True    ${cfg['target_configured']}
    Should Not Be Empty    ${cfg['runs']}
    ${sums} =    Target login checksum    ${restored_id}
    Should Be Equal    ${sums}    ${SUMS_ORIG}
    Secrets are kept out of the module environment    ${restored_id}
    The module may still configure itself    ${restored_id}
    Run on node    runagent -m ${restored_id} systemctl --user is-enabled passbolt-export.timer

The backup does not contain the Passbolt account or the KeePass secrets
    ${cfg} =    Run task    module/${restored_id}/get-configuration    {}
    Should Not Be True    ${cfg['private_key_set']}
    Should Not Be True    ${cfg['passphrase_set']}
    Should Not Be True    ${cfg['totp_secret_set']}
    Should Not Be True    ${cfg['server_pinned']}
    Should Not Be True    ${cfg['kdbx_password_set']}
    Should Not Be True    ${cfg['keyfile_set']}
    Run task    module/${restored_id}/run-export    {}    rc_expected=1    decode_json=${FALSE}
    ${cfg} =    Run task    module/${restored_id}/get-configuration    {}
    Should Contain    ${cfg['runs'][0]['error']}    not configured completely

The secrets can be entered again
    ${cfg} =    Config    retention=${7}
    Run task    module/${restored_id}/configure-module    ${cfg}    decode_json=${FALSE}
    ${cfg} =    Run task    module/${restored_id}/get-configuration    {}
    Should Be True    ${cfg['private_key_set']}
    Should Be True    ${cfg['passphrase_set']}
    Should Be True    ${cfg['kdbx_password_set']}
    Should Be True    ${cfg['keyfile_enabled']}
    Should Be True    ${cfg['keyfile_set']}
    Should Be True    ${cfg['target_configured']}

*** Keywords ***
Target login checksum
    [Arguments]    ${mid}
    ${out} =    Run on node    runagent -m ${mid} bash -c 'cd "$AGENT_STATE_DIR" && grep -vE "^(token|pass) " rclone/rclone.conf | sha256sum | cut -c1-64'
    RETURN    ${out}
