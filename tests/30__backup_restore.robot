*** Settings ***
Library     SSHLibrary
Resource    api.resource

*** Test Cases ***
Back up the module
    ${sums} =    Secret checksums    ${module_id}
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

The restored instance has the settings, the secrets and the history
    ${cfg} =    Run task    module/${restored_id}/get-configuration    {}
    Should Be Equal    ${cfg['passbolt_url']}    https://passbolt.ci.test:9
    Should Be Equal As Integers    ${cfg['retention']}    7
    Should Be Equal As Integers    ${cfg['schedule_monthday']}    15
    Should Be True    ${cfg['passphrase_set']}
    Should Be True    ${cfg['keyfile_set']}
    Should Be True    ${cfg['target_configured']}
    Should Not Be Empty    ${cfg['runs']}
    ${sums} =    Secret checksums    ${restored_id}
    Should Be Equal    ${sums}    ${SUMS_ORIG}
    Secrets are kept out of the module environment    ${restored_id}
    Run on node    runagent -m ${restored_id} systemctl --user is-enabled passbolt-export.timer

*** Keywords ***
Secret checksums
    [Arguments]    ${mid}
    ${out} =    Run on node    runagent -m ${mid} bash -c 'cd "$AGENT_STATE_DIR" && sha256sum passwords.env | cut -c1-64; grep -vE "^(token|pass) " rclone/rclone.conf | sha256sum | cut -c1-64'
    RETURN    ${out}
