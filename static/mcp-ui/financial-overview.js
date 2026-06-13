import { c as d, j as e } from "./client-BV8LgGti.js";
import { S as n } from "./StatCard-CN26b5Ll.js";
(function() {
  var t = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  t && typeof t.process > "u" && (t.process = { env: { NODE_ENV: "production" } });
})();
function u() {
  var s;
  const t = ((s = window.openai) == null ? void 0 : s.toolOutput) || {}, i = t.net_worth || 0, l = t.total_assets || 0, a = t.total_liabilities || 0, o = (r) => `$${(r / 100).toFixed(2)}`;
  return /* @__PURE__ */ e.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ e.jsx("h2", { style: { marginTop: 0 }, children: "Financial Overview" }),
    /* @__PURE__ */ e.jsx(
      n,
      {
        value: o(i),
        label: "Net Worth",
        trend: null
      }
    ),
    /* @__PURE__ */ e.jsxs("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "16px", marginTop: "16px" }, children: [
      /* @__PURE__ */ e.jsx(
        n,
        {
          value: o(l),
          label: "Total Assets",
          trend: null
        }
      ),
      /* @__PURE__ */ e.jsx(
        n,
        {
          value: o(a),
          label: "Total Liabilities",
          trend: null
        }
      )
    ] })
  ] });
}
if (typeof window < "u") {
  const t = document.getElementById("root");
  t && d(t).render(/* @__PURE__ */ e.jsx(u, {}));
}
export {
  u as default
};
