*** Settings ***
Library     SSHLibrary
Resource    api.resource

*** Test Cases ***
Clone the module
    ${out} =    Run task    cluster/clone-module    {"module":"${restored_id}","replace":false,"node":1}
    Set Global Variable    ${clone_id}    ${out['module_id']}
    Should Not Be Equal    ${clone_id}    ${restored_id}

The clone has the settings and the secrets
    ${cfg} =    Run task    module/${clone_id}/get-configuration    {}
    Should Be Equal    ${cfg['passbolt_url']}    https://passbolt.ci.test:9
    Should Be Equal As Integers    ${cfg['retention']}    7
    Should Be Equal    ${cfg['file_prefix']}    ci-export
    Should Be True    ${cfg['passphrase_set']}
    Should Be True    ${cfg['target_configured']}
    Secrets are kept out of the module environment    ${clone_id}
    Run on node    runagent -m ${clone_id} systemctl --user is-enabled passbolt-export.timer

Remove the clone
    Run on node    remove-module --no-preserve ${clone_id}
