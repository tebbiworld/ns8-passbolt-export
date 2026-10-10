//
// Copyright (C) 2023 Nethesis S.r.l.
// SPDX-License-Identifier: GPL-3.0-or-later
//
import Vue from "vue";
import VueRouter from "vue-router";
import Status from "../views/Status.vue";
import Settings from "../views/Settings.vue";
import Guide from "../views/Guide.vue";
import Sheet from "../views/Sheet.vue";

Vue.use(VueRouter);

const routes = [
  {
    path: "/",
    name: "Status",
    component: Status,
    alias: "/status", // important
  },
  {
    path: "/settings",
    name: "Settings",
    component: Settings,
  },
  {
    path: "/guide",
    name: "Guide",
    component: Guide,
  },
  {
    path: "/sheet",
    name: "Sheet",
    component: Sheet,
  },
  {
    path: "/about",
    name: "About",
    component: () =>
      import(/* webpackChunkName: "about" */ "../views/About.vue"),
  },
];

const router = new VueRouter({
  base: process.env.BASE_URL,
  routes,
});

export default router;
