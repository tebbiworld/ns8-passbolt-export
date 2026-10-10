<!--
  Copyright (C) 2026 tebbi
  SPDX-License-Identifier: GPL-3.0-or-later
-->
<template>
  <cv-grid fullWidth>
    <cv-row>
      <cv-column class="page-title">
        <h2>{{ $t("guide.title") }}</h2>
      </cv-column>
    </cv-row>
    <cv-row>
      <cv-column :md="8" :max="12">
        <cv-tile light class="tile">
          <h4>{{ $t("guide.why_title") }}</h4>
          <p class="help">{{ $t("guide.why_intro") }}</p>
          <ol class="steps">
            <li v-for="n in 4" :key="n">{{ $t("guide.why_" + n) }}</li>
          </ol>
          <p class="help">{{ $t("guide.why_break") }}</p>
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.kit_title") }}</h4>
          <p class="help">{{ $t("guide.kit_help") }}</p>
          <ul class="bullets">
            <li v-for="n in 5" :key="n">{{ $t("guide.kit_" + n) }}</li>
          </ul>
          <NsInlineNotification
            kind="warning"
            :title="$t('guide.kit_warning_title')"
            :description="$t('guide.kit_warning')"
            :showCloseButton="false"
          />
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.open_title") }}</h4>
          <ol class="steps">
            <li v-for="n in 5" :key="n">{{ $t("guide.open_" + n) }}</li>
          </ol>
          <h5 class="sub">{{ $t("guide.checksum_title") }}</h5>
          <p class="help">{{ $t("guide.checksum_help") }}</p>
          <pre class="code">{{ checksumCommands }}</pre>
          <h5 class="sub">{{ $t("guide.apps_title") }}</h5>
          <dl class="options">
            <template v-for="app in apps">
              <dt :key="app + '-t'">{{ $t("guide.app_" + app + "_name") }}</dt>
              <dd :key="app + '-d'">{{ $t("guide.app_" + app) }}</dd>
            </template>
          </dl>
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.account_title") }}</h4>
          <p class="help">{{ $t("guide.account_help") }}</p>
          <ol class="steps">
            <li v-for="n in 6" :key="n">{{ $t("guide.account_" + n) }}</li>
          </ol>
          <p class="help">{{ $t("guide.account_risk") }}</p>
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.target_title") }}</h4>
          <p class="help">{{ $t("guide.target_help") }}</p>
          <pre class="code">{{ authorizeCommands }}</pre>
          <ul class="bullets">
            <li v-for="n in 3" :key="n">{{ $t("guide.target_" + n) }}</li>
          </ul>
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.test_title") }}</h4>
          <ul class="bullets">
            <li v-for="n in 4" :key="n">{{ $t("guide.test_" + n) }}</li>
          </ul>
        </cv-tile>

        <cv-tile light class="tile">
          <h4>{{ $t("guide.limits_title") }}</h4>
          <ul class="bullets">
            <li v-for="n in 5" :key="n">{{ $t("guide.limits_" + n) }}</li>
          </ul>
        </cv-tile>
      </cv-column>
    </cv-row>
  </cv-grid>
</template>

<script>
import { mapState } from "vuex";
import {
  QueryParamService,
  UtilService,
  PageTitleService,
} from "@nethserver/ns8-ui-lib";

export default {
  name: "Guide",
  mixins: [UtilService, QueryParamService, PageTitleService],
  pageTitle() {
    return this.$t("guide.title") + " - " + this.appName;
  },
  data() {
    return {
      q: { page: "guide" },
      urlCheckInterval: null,
      apps: ["keepassxc", "strongbox", "keepassdx", "keeweb"],
      checksumCommands: [
        "# Windows (PowerShell)",
        "Get-FileHash .\\passbolt-export-2026-10-11_033000.kdbx -Algorithm SHA256",
        "# macOS",
        "shasum -a 256 passbolt-export-2026-10-11_033000.kdbx",
        "# Linux",
        "sha256sum passbolt-export-2026-10-11_033000.kdbx",
      ].join("\n"),
      authorizeCommands: [
        'rclone authorize "dropbox"',
        'rclone authorize "onedrive"',
        'rclone authorize "drive"',
        'rclone authorize "pcloud"',
      ].join("\n"),
    };
  },
  computed: {
    ...mapState(["instanceName", "core", "appName"]),
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
};
</script>

<style scoped lang="scss">
@import "../styles/carbon-utils";
.tile {
  margin-bottom: $spacing-06;
}
h4 {
  margin-bottom: $spacing-04;
}
.sub {
  margin: $spacing-07 0 $spacing-03 0;
}
.help {
  margin-bottom: $spacing-05;
}
.steps,
.bullets {
  margin: 0 0 $spacing-05 $spacing-06;
  li {
    margin-bottom: $spacing-03;
  }
}
.steps {
  list-style: decimal;
}
.bullets {
  list-style: disc;
}
.options {
  margin-bottom: $spacing-04;
  dt {
    font-weight: 600;
    margin-top: $spacing-03;
  }
  dd {
    margin: $spacing-02 0 0 0;
  }
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
</style>
