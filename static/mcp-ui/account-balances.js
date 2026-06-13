import { c as w, j as n } from "./client-BV8LgGti.js";
import { S as i } from "./StatCard-CN26b5Ll.js";
(function() {
  var t = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  t && typeof t.process > "u" && (t.process = { env: { NODE_ENV: "production" } });
})();
function x() {
  var l, c;
  const t = ((l = window.openai) == null ? void 0 : l.toolOutput) || {}, r = (((c = window.openai) == null ? void 0 : c.toolResponseMetadata) || {}).raw_values || {}, s = (e) => {
    if (typeof r[e] == "number")
      return r[e];
    if (typeof t[e] == "number")
      return t[e];
    const o = e.replace("_cents", "");
    return typeof t[o] == "number" ? t[o] : 0;
  }, d = (e = 0) => `$${(e / 100).toFixed(2)}`, a = (e, o) => typeof e == "string" && e.trim().length > 0 ? e : d(o), p = s("net_worth_cents"), f = s("total_assets_cents"), m = s("total_liabilities_cents"), y = a(t.net_worth, p), b = a(t.total_assets, f), g = a(t.total_liabilities, m);
  return /* @__PURE__ */ n.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ n.jsx("h2", { style: { marginTop: 0 }, children: "Account Balances" }),
    /* @__PURE__ */ n.jsx(
      i,
      {
        value: y,
        label: "Net Worth",
        trend: null
      }
    ),
    /* @__PURE__ */ n.jsxs("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "16px", marginTop: "16px" }, children: [
      /* @__PURE__ */ n.jsx(
        i,
        {
          value: b,
          label: "Total Assets",
          trend: null
        }
      ),
      /* @__PURE__ */ n.jsx(
        i,
        {
          value: g,
          label: "Total Liabilities",
          trend: null
        }
      )
    ] })
  ] });
}
if (typeof window < "u") {
  const t = document.getElementById("root");
  t && w(t).render(/* @__PURE__ */ n.jsx(x, {}));
}
export {
  x as default
};
