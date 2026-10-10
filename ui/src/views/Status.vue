<!--
  Copyright (C) 2026 tebbi
  SPDX-License-Identifier: GPL-3.0-or-later
-->
<template>
  <cv-grid fullWidth>
    <cv-row>
      <cv-column class="page-title">
        <h2>{{ $t("status.title") }}</h2>
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
    <cv-row v-if="!loading.getConfiguration && missing.length">
      <cv-column>
        <NsInlineNotification
          kind="warning"
          :title="$t('status.incomplete_title')"
          :description="
            $t('status.incomplete', {
              items: missing.map((m) => $t('status.missing_' + m)).join(', '),
            })
          "
          :actionLabel="$t('settings.title')"
          @action="goToAppPage(instanceName, 'settings')"
          :showCloseButton="false"
        />
      </cv-column>
    </cv-row>
    <cv-row>
      <cv-column :md="4" :max="8">
        <cv-tile light class="tile">
          <h4 class="section-title">{{ $t("status.last_run") }}</h4>
          <cv-skeleton-text
            v-if="loading.getConfiguration"
            :paragraph="true"
            :line-count="5"
          />
          <div v-else-if="!last" class="muted">{{ $t("status.never") }}</div>
          <template v-else>
            <NsInlineNotification
              :kind="last.ok ? 'success' : 'error'"
              :title="last.ok ? $t('status.run_ok') : $t('status.run_failed')"
              :description="last.ok ? formatTime(last.time) : last.error"
              :showCloseButton="false"
            />
            <dl class="facts">
              <template v-for="f in lastFacts">
                <dt :key="f.key + '-k'">{{ $t("status.fact_" + f.key) }}</dt>
                <dd :key="f.key + '-v'" :class="{ mono: f.mono }">
                  {{ f.value }}
                </dd>
              </template>
            </dl>
          </template>
          <div class="buttons">
            <NsButton
              kind="primary"
              :icon="Upload20"
              :loading="loading.runExport"
              :disabled="busy || missing.length > 0"
              @click="runExport"
              >{{ $t("status.export_now") }}</NsButton
            >
            <NsButton
              kind="secondary"
              :icon="Connect20"
              :loading="loading.check"
              :disabled="busy || !accountReady"
              @click="checkConnection(false)"
              >{{ $t("status.check") }}</NsButton
            >
          </div>
          <NsInlineNotification
            v-if="error.runExport"
            kind="error"
            :title="$t('action.run-export')"
            :description="error.runExport"
            :showCloseButton="false"
          />
        </cv-tile>
      </cv-column>
      <cv-column :md="4" :max="8">
        <cv-tile light class="tile">
          <h4 class="section-title">{{ $t("status.overview") }}</h4>
          <dl class="facts">
            <dt>{{ $t("status.passbolt") }}</dt>
            <dd>{{ config.passbolt_url || "-" }}</dd>
            <dt>{{ $t("status.account") }}</dt>
            <dd>
              {{ config.passbolt_user || $t("status.not_checked") }}
              <span v-if="config.server_pinned" class="muted">
                · {{ $t("status.pinned") }}</span
              >
            </dd>
            <dt>{{ $t("status.target") }}</dt>
            <dd>
              {{
                config.target_type
                  ? $t("target." + config.target_type) +
                    " · " +
                    (config.target_path || "/")
                  : "-"
              }}
            </dd>
            <dt>{{ $t("status.protection") }}</dt>
            <dd>
              {{
                config.keyfile_enabled
                  ? $t("status.password_and_keyfile")
                  : $t("status.password_only")
              }}
            </dd>
            <dt>{{ $t("status.next_run") }}</dt>
            <dd>
              {{
                config.next_run
                  ? formatTime(config.next_run)
                  : $t("status.no_schedule")
              }}
            </dd>
            <dt>{{ $t("status.retention") }}</dt>
            <dd>{{ config.retention }}</dd>
          </dl>
          <template v-if="check">
            <NsInlineNotification
              :kind="check.ok ? 'success' : 'error'"
              :title="
                check.ok ? $t('status.check_ok') : $t('status.check_failed')
              "
              :description="checkText"
              :showCloseButton="false"
            />
            <NsButton
              v-if="!check.ok && /server identity/.test(check.error || '')"
              kind="danger--tertiary"
              size="sm"
              :disabled="busy"
              @click="checkConnection(true)"
              >{{ $t("status.repin") }}</NsButton
            >
          </template>
          <NsInlineNotification
            v-if="error.check"
            kind="error"
            :title="$t('action.check-connection')"
            :description="error.check"
            :showCloseButton="false"
          />
        </cv-tile>
      </cv-column>
    </cv-row>
    <cv-row v-if="runs.length">
      <cv-column>
        <cv-tile light class="tile">
          <h4 class="section-title">{{ $t("status.history") }}</h4>
          <table class="history">
            <thead>
              <tr>
                <th>{{ $t("status.col_time") }}</th>
                <th>{{ $t("status.col_result") }}</th>
                <th>{{ $t("status.col_trigger") }}</th>
                <th>{{ $t("status.col_entries") }}</th>
                <th>{{ $t("status.col_file") }}</th>
                <th>{{ $t("status.col_mail") }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in runs" :key="r.time + '-' + r.trigger">
                <td>{{ formatTime(r.time) }}</td>
                <td :class="r.ok ? 'ok' : 'failed'" :title="r.error || ''">
                  {{ r.ok ? $t("status.ok") : $t("status.failed") }}
                </td>
                <td>{{ $t("status.trigger_" + r.trigger) }}</td>
                <td>{{ r.ok ? r.entries : "-" }}</td>
                <td class="mono">{{ r.file_name || "-" }}</td>
                <td>{{ $t("status.mail_" + (r.mail || "skipped")) }}</td>
              </tr>
            </tbody>
          </table>
        </cv-tile>
      </cv-column>
    </cv-row>
  </cv-grid>
</template>

<script>
import { mapState } from "vuex";
import Upload20 from "@carbon/icons-vue/es/upload/20";
import Connect20 from "@carbon/icons-vue/es/connect/20";
import {
  QueryParamService,
  UtilService,
  IconService,
  PageTitleService,
} from "@nethserver/ns8-ui-lib";
import moduleTask from "@/mixins/moduleTask";

export default {
  name: "Status",
  mixins: [
    moduleTask,
    IconService,
    UtilService,
    QueryParamService,
    PageTitleService,
  ],
  pageTitle() {
    return this.$t("status.title") + " - " + this.appName;
  },
  data() {
    return {
      q: { page: "status" },
      urlCheckInterval: null,
      Upload20,
      Connect20,
      config: {},
      runs: [],
      check: null,
      loading: { getConfiguration: false, runExport: false, check: false },
      error: { getConfiguration: "", runExport: "", check: "" },
    };
  },
  computed: {
    ...mapState(["instanceName", "core", "appName"]),
    busy() {
      return (
        this.loading.getConfiguration ||
        this.loading.runExport ||
        this.loading.check
      );
    },
    last() {
      return this.runs[0] || null;
    },
    accountReady() {
      const c = this.config;
      return !!(c.passbolt_url && c.private_key_set && c.passphrase_set);
    },
    missing() {
      const c = this.config;
      if (!c.retention) return [];
      const m = [];
      if (!this.accountReady) m.push("account");
      if (!c.kdbx_password_set) m.push("kdbx_password");
      if (c.keyfile_enabled && !c.keyfile_set) m.push("keyfile");
      if (!c.target_configured) m.push("target");
      return m;
    },
    lastFacts() {
      const r = this.last;
      if (!r) return [];
      const facts = [
        { key: "time", value: this.formatTime(r.time) },
        { key: "trigger", value: this.$t("status.trigger_" + r.trigger) },
      ];
      if (r.ok) {
        facts.push(
          {
            key: "entries",
            value: this.$t("status.entries_value", {
              entries: r.entries,
              resources: r.resources,
              folders: r.folders,
            }),
          },
          { key: "file", value: r.file_name, mono: true },
          { key: "size", value: this.formatBytes(r.size) },
          { key: "sha256", value: r.sha256, mono: true },
          {
            key: "verified",
            value: r.verified ? this.$t("status.yes") : this.$t("status.no"),
          },
          { key: "kept", value: r.kept }
        );
        if (r.skipped) {
          facts.push({
            key: "skipped",
            value: this.$t("status.skipped_value", { n: r.skipped }),
          });
        }
      }
      facts.push({
        key: "mail",
        value: this.$t("status.mail_" + (r.mail || "skipped")),
      });
      return facts;
    },
    checkText() {
      const c = this.check;
      if (!c) return "";
      if (!c.ok) return c.error;
      let text = this.$t("status.check_text", {
        user: c.user,
        resources: c.resources,
        folders: c.folders,
      });
      if (c.target_checked) {
        text += " " + this.$t("status.check_target_ok");
      }
      if (c.pinned_now) {
        text += " " + this.$t("status.check_pinned_now");
      }
      return text;
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
    formatTime(epoch) {
      if (!epoch) return "-";
      return new Date(epoch * 1000).toLocaleString();
    },
    formatBytes(n) {
      if (n === undefined || n === null) return "-";
      if (n < 1024) return n + " B";
      if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KiB";
      return (n / 1024 / 1024).toFixed(1) + " MiB";
    },
    async getConfiguration() {
      this.loading.getConfiguration = true;
      this.error.getConfiguration = "";
      try {
        this.config = await this.runModuleTask("get-configuration");
        this.runs = this.config.runs || [];
      } catch (e) {
        this.error.getConfiguration = this.$t("error.generic_error");
      } finally {
        this.loading.getConfiguration = false;
      }
    },
    async runExport() {
      this.loading.runExport = true;
      this.error.runExport = "";
      try {
        await this.runModuleTask(
          "run-export",
          {},
          { hidden: false, title: this.$t("action.run-export") }
        );
      } catch (e) {
        // the failed run is recorded in the history like a scheduled one
        if (!e.aborted) {
          this.error.runExport = this.$t("error.generic_error");
        }
      } finally {
        this.loading.runExport = false;
        this.getConfiguration();
      }
    },
    async checkConnection(repin) {
      this.loading.check = true;
      this.error.check = "";
      this.check = null;
      try {
        this.check = await this.runModuleTask("check-connection", {
          repin: !!repin,
        });
      } catch (e) {
        this.error.check = e.validation
          ? this.$t("status.missing_account")
          : this.$t("error.generic_error");
      } finally {
        this.loading.check = false;
        this.getConfiguration();
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
  margin-bottom: $spacing-05;
}
.muted {
  color: $text-02;
}
.facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: $spacing-03 $spacing-06;
  margin: $spacing-05 0;
  dt {
    font-weight: 600;
  }
  dd {
    word-break: break-all;
  }
}
.mono {
  font-family: "IBM Plex Mono", monospace;
  font-size: 0.75rem;
}
.buttons {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-05;
  margin: $spacing-05 0;
}
.history {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
  th,
  td {
    text-align: left;
    padding: $spacing-03 $spacing-04;
    border-bottom: 1px solid $ui-03;
  }
  th {
    font-weight: 600;
  }
  .ok {
    color: $support-02;
  }
  .failed {
    color: $support-01;
  }
}
</style>
