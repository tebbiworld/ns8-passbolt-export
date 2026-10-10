<!--
  Copyright (C) 2026 tebbi
  SPDX-License-Identifier: GPL-3.0-or-later
-->
<template>
  <cv-grid fullWidth>
    <cv-row>
      <cv-column class="page-title">
        <h2>{{ $t("settings.title") }}</h2>
      </cv-column>
    </cv-row>
    <cv-row v-if="error.getConfiguration">
      <cv-column>
        <NsInlineNotification
          kind="error"
          :title="$t('action.get-configuration')"
          :description="error.getConfiguration"
          :showCloseButton="false"
        />
      </cv-column>
    </cv-row>
    <cv-form @submit.prevent="configureModule">
      <cv-row>
        <cv-column :md="4" :max="8">
          <!-- Passbolt account -->
          <cv-tile light class="tile">
            <h4 class="section-title">{{ $t("settings.account_title") }}</h4>
            <p class="help">{{ $t("settings.account_help") }}</p>
            <cv-dropdown
              v-if="instances.length"
              :label="$t('settings.instance')"
              v-model="instance"
              :disabled="busy"
              class="field"
            >
              <cv-dropdown-item
                v-for="i in instances"
                :key="i.module_id"
                :value="i.url"
                >{{ i.url }} ({{ i.module_id }})</cv-dropdown-item
              >
              <cv-dropdown-item value="">{{
                $t("settings.instance_other")
              }}</cv-dropdown-item>
            </cv-dropdown>
            <NsTextInput
              v-if="!instance"
              :label="$t('settings.passbolt_url')"
              v-model.trim="passbolt_url"
              placeholder="https://passbolt.example.com"
              :invalid-message="error.passbolt_url"
              :disabled="busy"
              ref="passbolt_url"
            />
            <div class="field">
              <div class="bx--label">{{ $t("settings.private_key") }}</div>
              <div class="file-row">
                <NsButton
                  kind="tertiary"
                  size="sm"
                  :icon="Upload20"
                  :disabled="busy"
                  @click.prevent="$refs.keyInput.click()"
                  >{{ $t("settings.private_key_choose") }}</NsButton
                >
                <span class="file-state">{{ privateKeyState }}</span>
              </div>
              <input
                ref="keyInput"
                type="file"
                accept=".asc,.txt,.key,.gpg"
                class="hidden-input"
                @change="loadPrivateKey"
              />
              <div class="bx--form__helper-text">
                {{ $t("settings.private_key_help") }}
              </div>
              <div v-if="error.passbolt_private_key" class="invalid">
                {{ error.passbolt_private_key }}
              </div>
            </div>
            <NsTextInput
              :label="$t('settings.passphrase')"
              type="password"
              v-model="passbolt_key_password"
              autocomplete="new-password"
              :helper-text="
                passphrase_set ? $t('settings.keep') : $t('settings.required')
              "
              :invalid-message="error.passbolt_key_password"
              :disabled="busy"
              ref="passbolt_key_password"
            />
            <NsTextInput
              :label="$t('settings.totp_secret')"
              type="password"
              v-model.trim="passbolt_totp_secret"
              autocomplete="off"
              :helper-text="
                totp_secret_set
                  ? $t('settings.totp_secret_set')
                  : $t('settings.totp_secret_help')
              "
              :invalid-message="error.passbolt_totp_secret"
              :disabled="busy"
              ref="passbolt_totp_secret"
            />
            <cv-checkbox
              v-if="totp_secret_set"
              :label="$t('settings.clear_totp_secret')"
              v-model="clear_totp_secret"
              value="clear"
              :disabled="busy || !!passbolt_totp_secret"
              class="field"
            />
            <cv-text-area
              :label="$t('settings.ca_pem')"
              v-model.trim="passbolt_ca_pem"
              :helper-text="$t('settings.ca_pem_help')"
              :invalid-message="error.passbolt_ca_pem"
              :disabled="busy"
              class="field mono"
              :rows="3"
              placeholder="-----BEGIN CERTIFICATE-----"
            />
            <div v-if="passbolt_user" class="muted field">
              {{
                $t("settings.account_user", {
                  user: passbolt_user,
                })
              }}
              <span v-if="server_pinned">{{
                $t("settings.server_pinned")
              }}</span>
            </div>
          </cv-tile>

          <!-- KeePass protection -->
          <cv-tile light class="tile">
            <h4 class="section-title">{{ $t("settings.kdbx_title") }}</h4>
            <p class="help">{{ $t("settings.kdbx_help") }}</p>
            <NsTextInput
              :label="$t('settings.kdbx_password')"
              type="password"
              v-model="kdbx_password"
              autocomplete="new-password"
              :helper-text="
                kdbx_password_set
                  ? $t('settings.keep')
                  : $t('settings.kdbx_password_help')
              "
              :invalid-message="error.kdbx_password"
              :disabled="busy"
              ref="kdbx_password"
            />
            <NsTextInput
              v-if="kdbx_password"
              :label="$t('settings.kdbx_password_repeat')"
              type="password"
              v-model="kdbx_password_repeat"
              autocomplete="new-password"
              :invalid-message="error.kdbx_password_repeat"
              :disabled="busy"
            />
            <NsToggle
              :label="$t('settings.keyfile_enabled')"
              value="keyfile"
              :form-item="true"
              v-model="keyfile_enabled"
              :disabled="busy"
              class="field"
            >
              <template slot="text-left">{{ $t("settings.off") }}</template>
              <template slot="text-right">{{ $t("settings.on") }}</template>
            </NsToggle>
            <template v-if="keyfile_enabled">
              <p class="help">{{ $t("settings.keyfile_help") }}</p>
              <div class="file-row field">
                <NsButton
                  kind="secondary"
                  size="sm"
                  :icon="Download20"
                  :disabled="busy"
                  @click.prevent="generateKeyfile"
                  >{{
                    keyfile_set || kdbx_keyfile_secret
                      ? $t("settings.keyfile_new")
                      : $t("settings.keyfile_generate")
                  }}</NsButton
                >
                <span class="file-state">{{ keyfileState }}</span>
              </div>
              <div v-if="error.kdbx_keyfile_secret" class="invalid field">
                {{ error.kdbx_keyfile_secret }}
              </div>
              <NsInlineNotification
                v-if="kdbx_keyfile_secret"
                kind="warning"
                :title="$t('settings.keyfile_unsaved_title')"
                :description="$t('settings.keyfile_unsaved')"
                :showCloseButton="false"
              />
            </template>
            <NsInlineNotification
              kind="info"
              :title="$t('settings.offline_title')"
              :description="$t('settings.offline')"
              :showCloseButton="false"
            />
          </cv-tile>
        </cv-column>

        <cv-column :md="4" :max="8">
          <!-- target -->
          <cv-tile light class="tile">
            <h4 class="section-title">{{ $t("settings.target_title") }}</h4>
            <cv-dropdown
              :label="$t('settings.target_type')"
              v-model="target_type"
              :disabled="busy"
              class="field"
            >
              <cv-dropdown-item value="">{{
                $t("settings.target_none")
              }}</cv-dropdown-item>
              <cv-dropdown-item v-for="t in targetTypes" :key="t" :value="t">{{
                $t("target." + t)
              }}</cv-dropdown-item>
            </cv-dropdown>
            <template v-if="target_type">
              <NsTextInput
                :label="$t('settings.target_path')"
                v-model.trim="target_path"
                :helper-text="$t('settings.target_path_help_' + pathHelp)"
                :invalid-message="error.target_path"
                :disabled="busy"
                ref="target_path"
              />
              <template v-if="isOauth">
                <p class="help">
                  {{ $t("settings.oauth_help") }}
                </p>
                <pre class="code">rclone authorize "{{ target_type }}"</pre>
                <p class="help">{{ $t("settings.oauth_help_2") }}</p>
                <cv-text-area
                  :label="$t('settings.target_token')"
                  v-model.trim="target_token"
                  :helper-text="
                    target_secret_set
                      ? $t('settings.keep')
                      : $t('settings.required')
                  "
                  :invalid-message="error.target_token"
                  :disabled="busy"
                  class="field mono"
                  :rows="3"
                  placeholder='{"access_token":"…","token_type":"bearer",…}'
                />
                <cv-dropdown
                  v-if="target_type === 'pcloud'"
                  :label="$t('settings.pcloud_hostname')"
                  v-model="pcloud_hostname"
                  :disabled="busy"
                  class="field"
                >
                  <cv-dropdown-item value="eapi.pcloud.com">{{
                    $t("settings.pcloud_eu")
                  }}</cv-dropdown-item>
                  <cv-dropdown-item value="api.pcloud.com">{{
                    $t("settings.pcloud_us")
                  }}</cv-dropdown-item>
                </cv-dropdown>
              </template>
              <template v-if="target_type === 's3'">
                <cv-dropdown
                  :label="$t('settings.s3_provider')"
                  v-model="s3_provider"
                  :disabled="busy"
                  class="field"
                >
                  <cv-dropdown-item
                    v-for="p in s3Providers"
                    :key="p"
                    :value="p"
                    >{{
                      p === "Other" ? $t("settings.s3_other") : p
                    }}</cv-dropdown-item
                  >
                </cv-dropdown>
                <NsTextInput
                  :label="$t('settings.s3_endpoint')"
                  v-model.trim="s3_endpoint"
                  placeholder="https://s3.example.com"
                  :invalid-message="error.s3_endpoint"
                  :disabled="busy"
                />
                <NsTextInput
                  :label="$t('settings.s3_region')"
                  v-model.trim="s3_region"
                  :disabled="busy"
                />
                <NsTextInput
                  :label="$t('settings.s3_access_key')"
                  type="password"
                  v-model.trim="s3_access_key"
                  autocomplete="off"
                  :helper-text="
                    target_secret_set
                      ? $t('settings.keep')
                      : $t('settings.required')
                  "
                  :invalid-message="error.s3_access_key"
                  :disabled="busy"
                />
                <NsTextInput
                  :label="$t('settings.s3_secret_access_key')"
                  type="password"
                  v-model.trim="s3_secret_access_key"
                  autocomplete="off"
                  :helper-text="
                    target_secret_set
                      ? $t('settings.keep')
                      : $t('settings.required')
                  "
                  :invalid-message="error.s3_secret_access_key"
                  :disabled="busy"
                />
              </template>
              <template v-if="target_type === 'webdav'">
                <NsTextInput
                  :label="$t('settings.webdav_url')"
                  v-model.trim="webdav_url"
                  placeholder="https://dav.example.com/remote.php/dav/files/user"
                  :invalid-message="error.webdav_url"
                  :disabled="busy"
                />
                <cv-dropdown
                  :label="$t('settings.webdav_vendor')"
                  v-model="webdav_vendor"
                  :disabled="busy"
                  class="field"
                >
                  <cv-dropdown-item
                    v-for="v in webdavVendors"
                    :key="v"
                    :value="v"
                    >{{ $t("settings.webdav_" + v) }}</cv-dropdown-item
                  >
                </cv-dropdown>
              </template>
              <template v-if="target_type === 'smb'">
                <NsTextInput
                  :label="$t('settings.smb_host')"
                  v-model.trim="smb_host"
                  placeholder="nas.example.com"
                  :invalid-message="error.smb_host"
                  :disabled="busy"
                />
                <NsTextInput
                  :label="$t('settings.smb_domain')"
                  v-model.trim="smb_domain"
                  :disabled="busy"
                />
              </template>
              <template
                v-if="target_type === 'webdav' || target_type === 'smb'"
              >
                <NsTextInput
                  :label="$t('settings.target_user')"
                  v-model.trim="target_user"
                  :invalid-message="error.target_user"
                  :disabled="busy"
                />
                <NsTextInput
                  :label="$t('settings.target_password')"
                  type="password"
                  v-model="target_password"
                  autocomplete="new-password"
                  :helper-text="
                    target_secret_set
                      ? $t('settings.keep')
                      : $t('settings.required')
                  "
                  :invalid-message="error.target_password"
                  :disabled="busy"
                />
              </template>
              <NsTextInput
                :label="$t('settings.retention')"
                v-model.number="retention"
                type="number"
                min="1"
                max="1000"
                :helper-text="$t('settings.retention_help')"
                :invalid-message="error.retention"
                :disabled="busy"
              />
              <NsTextInput
                :label="$t('settings.file_prefix')"
                v-model.trim="file_prefix"
                :helper-text="
                  $t('settings.file_prefix_help', { example: exampleName })
                "
                :invalid-message="error.file_prefix"
                :disabled="busy"
              />
            </template>
          </cv-tile>

          <!-- schedule and report -->
          <cv-tile light class="tile">
            <h4 class="section-title">{{ $t("settings.schedule_title") }}</h4>
            <NsToggle
              :label="$t('settings.schedule_enabled')"
              value="schedule"
              :form-item="true"
              v-model="schedule_enabled"
              :disabled="busy"
              class="field"
            >
              <template slot="text-left">{{ $t("settings.off") }}</template>
              <template slot="text-right">{{ $t("settings.on") }}</template>
            </NsToggle>
            <template v-if="schedule_enabled">
              <cv-dropdown
                :label="$t('settings.frequency')"
                v-model="schedule_frequency"
                :disabled="busy"
                class="field"
              >
                <cv-dropdown-item
                  v-for="f in ['daily', 'weekly', 'monthly']"
                  :key="f"
                  :value="f"
                  >{{ $t("settings.frequency_" + f) }}</cv-dropdown-item
                >
              </cv-dropdown>
              <cv-dropdown
                v-if="schedule_frequency === 'weekly'"
                :label="$t('settings.weekday')"
                v-model="schedule_weekday"
                :disabled="busy"
                class="field"
              >
                <cv-dropdown-item v-for="d in weekdays" :key="d" :value="d">{{
                  $t("weekday." + d)
                }}</cv-dropdown-item>
              </cv-dropdown>
              <NsTextInput
                v-if="schedule_frequency === 'monthly'"
                :label="$t('settings.monthday')"
                v-model.number="schedule_monthday"
                type="number"
                min="1"
                max="28"
                :helper-text="$t('settings.monthday_help')"
                :invalid-message="error.schedule_monthday"
                :disabled="busy"
              />
              <NsTextInput
                :label="$t('settings.time')"
                v-model.trim="schedule_time"
                placeholder="03:30"
                :helper-text="$t('settings.time_help')"
                :invalid-message="error.schedule_time"
                :disabled="busy"
              />
            </template>
            <h5 class="sub">{{ $t("settings.mail_title") }}</h5>
            <NsTextInput
              :label="$t('settings.notify_email')"
              v-model.trim="notify_email"
              placeholder="admin@example.com"
              :helper-text="$t('settings.notify_email_help')"
              :invalid-message="error.notify_email"
              :disabled="busy"
            />
            <template v-if="notify_email">
              <cv-dropdown
                :label="$t('settings.notify_mode')"
                v-model="notify_mode"
                :disabled="busy"
                class="field"
              >
                <cv-dropdown-item
                  v-for="m in ['always', 'failure', 'never']"
                  :key="m"
                  :value="m"
                  >{{ $t("settings.notify_" + m) }}</cv-dropdown-item
                >
              </cv-dropdown>
              <cv-dropdown
                :label="$t('settings.mail_language')"
                v-model="mail_language"
                :disabled="busy"
                class="field"
              >
                <cv-dropdown-item
                  v-for="l in ['en', 'de', 'it', 'fr']"
                  :key="l"
                  :value="l"
                  >{{ $t("language." + l) }}</cv-dropdown-item
                >
              </cv-dropdown>
              <NsTextInput
                :label="$t('settings.notify_from')"
                v-model.trim="notify_from"
                :helper-text="$t('settings.notify_from_help')"
                :invalid-message="error.notify_from"
                :disabled="busy"
              />
            </template>
          </cv-tile>
        </cv-column>
      </cv-row>
      <cv-row>
        <cv-column>
          <NsInlineNotification
            v-if="error.configureModule"
            kind="error"
            :title="$t('action.configure-module')"
            :description="error.configureModule"
            :showCloseButton="false"
          />
          <NsInlineNotification
            v-if="saved"
            kind="success"
            :title="$t('settings.saved_title')"
            :description="$t('settings.saved')"
            :showCloseButton="false"
          />
          <NsButton
            kind="primary"
            :icon="Save20"
            :loading="loading.configureModule"
            :disabled="busy"
            >{{ $t("settings.save") }}</NsButton
          >
        </cv-column>
      </cv-row>
    </cv-form>
  </cv-grid>
</template>

<script>
import { mapState } from "vuex";
import Upload20 from "@carbon/icons-vue/es/upload/20";
import Download20 from "@carbon/icons-vue/es/download/20";
import {
  QueryParamService,
  UtilService,
  IconService,
  PageTitleService,
} from "@nethserver/ns8-ui-lib";
import moduleTask from "@/mixins/moduleTask";

const SECRET_INPUTS = [
  "passbolt_private_key",
  "passbolt_key_password",
  "passbolt_totp_secret",
  "kdbx_password",
  "kdbx_password_repeat",
  "kdbx_keyfile_secret",
  "target_token",
  "s3_access_key",
  "s3_secret_access_key",
  "target_password",
];

export default {
  name: "Settings",
  mixins: [
    moduleTask,
    IconService,
    UtilService,
    QueryParamService,
    PageTitleService,
  ],
  pageTitle() {
    return this.$t("settings.title") + " - " + this.appName;
  },
  data() {
    return {
      q: { page: "settings" },
      urlCheckInterval: null,
      Upload20,
      Download20,
      targetTypes: [
        "dropbox",
        "onedrive",
        "drive",
        "pcloud",
        "s3",
        "webdav",
        "smb",
      ],
      s3Providers: [
        "Other",
        "AWS",
        "Minio",
        "Ceph",
        "Cloudflare",
        "Hetzner",
        "IDrive",
        "IONOS",
        "OVHcloud",
        "Scaleway",
        "Storj",
        "Synology",
        "Wasabi",
      ],
      webdavVendors: ["other", "nextcloud", "owncloud", "sharepoint"],
      weekdays: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
      instances: [],
      instance: "",
      passbolt_url: "",
      passbolt_ca_pem: "",
      passbolt_user: "",
      passbolt_private_key: "",
      privateKeyName: "",
      passbolt_key_password: "",
      passbolt_totp_secret: "",
      clear_totp_secret: false,
      private_key_set: false,
      passphrase_set: false,
      totp_secret_set: false,
      server_pinned: false,
      kdbx_password: "",
      kdbx_password_repeat: "",
      kdbx_password_set: false,
      keyfile_enabled: false,
      keyfile_set: false,
      kdbx_keyfile_secret: "",
      target_type: "",
      stored_target_type: "",
      target_path: "Passbolt",
      target_configured: false,
      target_token: "",
      pcloud_hostname: "eapi.pcloud.com",
      s3_provider: "Other",
      s3_endpoint: "",
      s3_region: "",
      s3_access_key: "",
      s3_secret_access_key: "",
      webdav_url: "",
      webdav_vendor: "other",
      smb_host: "",
      smb_domain: "",
      target_user: "",
      target_password: "",
      file_prefix: "passbolt-export",
      retention: 12,
      schedule_enabled: true,
      schedule_frequency: "weekly",
      schedule_weekday: "Sun",
      schedule_monthday: 1,
      schedule_time: "03:30",
      notify_email: "",
      notify_from: "",
      notify_mode: "always",
      mail_language: "en",
      saved: false,
      loading: {
        getConfiguration: false,
        configureModule: false,
      },
      error: {
        getConfiguration: "",
        configureModule: "",
        passbolt_url: "",
        passbolt_ca_pem: "",
        passbolt_private_key: "",
        passbolt_key_password: "",
        passbolt_totp_secret: "",
        kdbx_password: "",
        kdbx_password_repeat: "",
        kdbx_keyfile_secret: "",
        target_path: "",
        target_token: "",
        s3_endpoint: "",
        s3_access_key: "",
        s3_secret_access_key: "",
        webdav_url: "",
        smb_host: "",
        target_user: "",
        target_password: "",
        retention: "",
        file_prefix: "",
        schedule_monthday: "",
        schedule_time: "",
        notify_email: "",
        notify_from: "",
      },
    };
  },
  computed: {
    ...mapState(["instanceName", "core", "appName"]),
    busy() {
      return this.loading.getConfiguration || this.loading.configureModule;
    },
    isOauth() {
      return ["dropbox", "onedrive", "drive", "pcloud"].includes(
        this.target_type
      );
    },
    target_secret_set() {
      return (
        this.target_configured && this.stored_target_type === this.target_type
      );
    },
    pathHelp() {
      return ["s3", "smb"].includes(this.target_type)
        ? this.target_type
        : "folder";
    },
    privateKeyState() {
      if (this.privateKeyName) {
        return this.$t("settings.private_key_loaded", {
          name: this.privateKeyName,
        });
      }
      return this.private_key_set
        ? this.$t("settings.private_key_stored")
        : this.$t("settings.private_key_missing");
    },
    keyfileState() {
      if (this.kdbx_keyfile_secret) {
        return this.$t("settings.keyfile_generated");
      }
      return this.keyfile_set
        ? this.$t("settings.keyfile_stored")
        : this.$t("settings.keyfile_missing");
    },
    exampleName() {
      return (
        (this.file_prefix || "passbolt-export") + "-2026-10-11_033000.kdbx"
      );
    },
    effectiveUrl() {
      return this.instance || this.passbolt_url;
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
    next();
  },
  created() {
    this.getConfiguration();
  },
  methods: {
    clearErrors() {
      for (const k of Object.keys(this.error)) {
        this.error[k] = "";
      }
    },
    clearSecrets() {
      // never keep a typed secret in the page longer than needed
      for (const k of SECRET_INPUTS) {
        this[k] = "";
      }
      this.privateKeyName = "";
      this.clear_totp_secret = false;
      if (this.$refs.keyInput) {
        this.$refs.keyInput.value = "";
      }
    },
    async getConfiguration() {
      this.loading.getConfiguration = true;
      this.error.getConfiguration = "";
      try {
        const [c, list] = await Promise.all([
          this.runModuleTask("get-configuration"),
          this.runModuleTask("list-passbolt-instances").catch(() => ({
            instances: [],
          })),
        ]);
        this.instances = list.instances || [];
        for (const k of [
          "passbolt_url",
          "passbolt_ca_pem",
          "passbolt_user",
          "private_key_set",
          "passphrase_set",
          "totp_secret_set",
          "server_pinned",
          "kdbx_password_set",
          "keyfile_enabled",
          "keyfile_set",
          "target_type",
          "target_configured",
          "s3_endpoint",
          "s3_region",
          "webdav_url",
          "smb_host",
          "smb_domain",
          "target_user",
          "file_prefix",
          "retention",
          "schedule_enabled",
          "schedule_frequency",
          "schedule_weekday",
          "schedule_monthday",
          "schedule_time",
          "notify_email",
          "notify_from",
          "notify_mode",
          "mail_language",
        ]) {
          this[k] = c[k];
        }
        this.target_path = c.target_path || "Passbolt";
        this.pcloud_hostname = c.pcloud_hostname || "eapi.pcloud.com";
        this.s3_provider = c.s3_provider || "Other";
        this.webdav_vendor = c.webdav_vendor || "other";
        this.stored_target_type = c.target_type;
        const known = this.instances.find((i) => i.url === c.passbolt_url);
        if (known) {
          this.instance = known.url;
        } else if (!c.passbolt_url && this.instances.length) {
          this.instance = this.instances[0].url;
        } else {
          this.instance = "";
        }
        if (!c.notify_email && this.core.$i18n) {
          const lang = (this.core.$i18n.locale || "en").substring(0, 2);
          this.mail_language = ["de", "it", "fr"].includes(lang) ? lang : "en";
        }
      } catch (e) {
        this.error.getConfiguration = this.$t("error.generic_error");
      } finally {
        this.loading.getConfiguration = false;
      }
    },
    loadPrivateKey(ev) {
      const file = ev.target.files && ev.target.files[0];
      this.error.passbolt_private_key = "";
      if (!file) {
        return;
      }
      if (file.size > 65536) {
        this.error.passbolt_private_key = this.$t(
          "settings.error_private_key_invalid"
        );
        return;
      }
      const reader = new FileReader();
      reader.onload = () => {
        const text = String(reader.result || "");
        if (!text.includes("-----BEGIN PGP PRIVATE KEY BLOCK-----")) {
          this.error.passbolt_private_key = this.$t(
            "settings.error_private_key_invalid"
          );
          return;
        }
        this.passbolt_private_key = text;
        this.privateKeyName = file.name;
      };
      reader.readAsText(file);
    },
    toHex(bytes) {
      return Array.from(bytes, (b) => b.toString(16).padStart(2, "0"))
        .join("")
        .toUpperCase();
    },
    async generateKeyfile() {
      // KeePass key file format 2.0, created in the browser: the module
      // receives it once with the next save and never hands it out again.
      const key = new Uint8Array(32);
      window.crypto.getRandomValues(key);
      const digest = new Uint8Array(
        await window.crypto.subtle.digest("SHA-256", key)
      );
      const hex = this.toHex(key).match(/.{8}/g);
      const xml =
        '<?xml version="1.0" encoding="utf-8"?>\n' +
        "<KeyFile>\n\t<Meta>\n\t\t<Version>2.0</Version>\n\t</Meta>\n" +
        "\t<Key>\n" +
        `\t\t<Data Hash="${this.toHex(digest.slice(0, 4))}">\n` +
        `\t\t\t${hex.slice(0, 4).join(" ")}\n` +
        `\t\t\t${hex.slice(4).join(" ")}\n` +
        "\t\t</Data>\n\t</Key>\n</KeyFile>\n";
      const blob = new Blob([xml], { type: "application/xml" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = (this.file_prefix || "passbolt-export") + ".keyx";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(a.href);
      this.kdbx_keyfile_secret = xml;
    },
    validate() {
      let ok = true;
      if (this.kdbx_password) {
        if (this.kdbx_password.length < 12) {
          this.error.kdbx_password = this.$t(
            "settings.error_kdbx_password_too_short"
          );
          ok = false;
        } else if (this.kdbx_password !== this.kdbx_password_repeat) {
          this.error.kdbx_password_repeat = this.$t(
            "settings.error_kdbx_password_mismatch"
          );
          ok = false;
        }
      }
      if (!/^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(this.schedule_time)) {
        this.error.schedule_time = this.$t("settings.error_time_invalid");
        ok = false;
      }
      if (!/^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/.test(this.file_prefix)) {
        this.error.file_prefix = this.$t("settings.error_file_prefix_invalid");
        ok = false;
      }
      return ok;
    },
    async configureModule() {
      this.clearErrors();
      this.saved = false;
      if (!this.validate()) {
        return;
      }
      this.loading.configureModule = true;
      const data = {
        passbolt_url: this.effectiveUrl,
        passbolt_ca_pem: this.passbolt_ca_pem,
        passbolt_private_key: this.passbolt_private_key,
        passbolt_key_password: this.passbolt_key_password,
        passbolt_totp_secret: this.passbolt_totp_secret,
        clear_totp_secret: !!this.clear_totp_secret,
        kdbx_password: this.kdbx_password,
        keyfile_enabled: !!this.keyfile_enabled,
        kdbx_keyfile_secret: this.keyfile_enabled
          ? this.kdbx_keyfile_secret
          : "",
        target_type: this.target_type,
        target_path: this.target_path,
        target_token: this.isOauth ? this.target_token : "",
        pcloud_hostname:
          this.target_type === "pcloud" ? this.pcloud_hostname : "",
        s3_provider: this.target_type === "s3" ? this.s3_provider : "",
        s3_endpoint: this.target_type === "s3" ? this.s3_endpoint : "",
        s3_region: this.target_type === "s3" ? this.s3_region : "",
        s3_access_key: this.target_type === "s3" ? this.s3_access_key : "",
        s3_secret_access_key:
          this.target_type === "s3" ? this.s3_secret_access_key : "",
        webdav_url: this.target_type === "webdav" ? this.webdav_url : "",
        webdav_vendor: this.target_type === "webdav" ? this.webdav_vendor : "",
        smb_host: this.target_type === "smb" ? this.smb_host : "",
        smb_domain: this.target_type === "smb" ? this.smb_domain : "",
        target_user: ["webdav", "smb"].includes(this.target_type)
          ? this.target_user
          : "",
        target_password: ["webdav", "smb"].includes(this.target_type)
          ? this.target_password
          : "",
        file_prefix: this.file_prefix,
        retention: Number(this.retention) || 12,
        schedule_enabled: !!this.schedule_enabled,
        schedule_frequency: this.schedule_frequency,
        schedule_weekday: this.schedule_weekday,
        schedule_monthday: Number(this.schedule_monthday) || 1,
        schedule_time: this.schedule_time,
        notify_email: this.notify_email,
        notify_from: this.notify_from,
        notify_mode: this.notify_mode,
        mail_language: this.mail_language,
      };
      try {
        await this.runModuleTask("configure-module", data, {
          hidden: false,
          title: this.$t("action.configure-module"),
        });
        this.clearSecrets();
        this.saved = true;
        await this.getConfiguration();
      } catch (e) {
        if (e.validation) {
          for (const v of e.validation) {
            const field = v.field in this.error ? v.field : "configureModule";
            const key = "settings.error_" + v.error;
            this.error[field] = this.$te(key)
              ? this.$t(key)
              : this.$t("settings.error_invalid");
          }
          if (this.error.passbolt_url && this.instance) {
            this.error.configureModule = this.error.passbolt_url;
          }
        } else {
          this.error.configureModule = this.$t("error.generic_error");
        }
      } finally {
        this.loading.configureModule = false;
      }
    },
  },
};
</script>

<style scoped lang="scss">
@import "../styles/carbon-utils";
.tile {
  margin-bottom: $spacing-06;
}
.section-title {
  margin-bottom: $spacing-04;
}
.sub {
  margin: $spacing-06 0 $spacing-04 0;
}
.help {
  margin-bottom: $spacing-05;
}
.field {
  margin-bottom: $spacing-06;
}
.muted {
  color: $text-02;
}
.file-row {
  display: flex;
  align-items: center;
  gap: $spacing-05;
  margin-bottom: $spacing-03;
}
.file-state {
  font-size: 0.875rem;
}
.hidden-input {
  display: none;
}
.invalid {
  color: $support-01;
  font-size: 0.75rem;
  margin-top: $spacing-02;
}
.code {
  font-family: "IBM Plex Mono", monospace;
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-all;
  padding: $spacing-04;
  background: $ui-01;
  margin-bottom: $spacing-05;
}
.mono ::v-deep textarea {
  font-family: "IBM Plex Mono", monospace;
  font-size: 0.75rem;
}
</style>
