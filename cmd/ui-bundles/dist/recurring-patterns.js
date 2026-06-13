import { c as f, j as t } from "./client-BV8LgGti.js";
import { S as a } from "./StatCard-CN26b5Ll.js";
(function() {
  var n = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  n && typeof n.process > "u" && (n.process = { env: { NODE_ENV: "production" } });
})();
function m() {
  var r, d, l;
  const n = ((r = window.openai) == null ? void 0 : r.toolOutput) || {}, o = ((d = window.openai) == null ? void 0 : d.toolResponseMetadata) || {}, i = ((l = window.openai) == null ? void 0 : l.theme) || "light", c = n.count || 0, p = n.total_monthly || "$0.00", s = o.patterns || [];
  return /* @__PURE__ */ t.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ t.jsx("h2", { style: { marginTop: 0 }, children: "Recurring Patterns" }),
    /* @__PURE__ */ t.jsxs("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "16px" }, children: [
      /* @__PURE__ */ t.jsx(
        a,
        {
          value: c.toString(),
          label: "Active Subscriptions",
          trend: null
        }
      ),
      /* @__PURE__ */ t.jsx(
        a,
        {
          value: p,
          label: "Monthly Total",
          trend: null
        }
      )
    ] }),
    s.length > 0 && /* @__PURE__ */ t.jsxs("div", { style: { marginTop: "16px" }, children: [
      /* @__PURE__ */ t.jsx("h3", { children: "Subscriptions & Bills" }),
      s.map((e, u) => /* @__PURE__ */ t.jsxs("div", { style: {
        marginBottom: "8px",
        padding: "12px",
        background: i === "dark" ? "#303030" : "#F3F3F3",
        borderRadius: "8px"
      }, children: [
        /* @__PURE__ */ t.jsx("strong", { children: e.entity_name || "Unknown" }),
        /* @__PURE__ */ t.jsxs("div", { style: { fontSize: "14px", color: i === "dark" ? "#AFAFAF" : "#5D5D5D" }, children: [
          "$",
          ((e.avg_amount_cents || 0) / 100).toFixed(2),
          " / ",
          e.frequency || "month"
        ] })
      ] }, u))
    ] })
  ] });
}
if (typeof window < "u") {
  const n = document.getElementById("root");
  n && f(n).render(/* @__PURE__ */ t.jsx(m, {}));
}
export {
  m as default
};
