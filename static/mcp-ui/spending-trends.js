import { c as a, j as n } from "./client-BV8LgGti.js";
(function() {
  var e = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  e && typeof e.process > "u" && (e.process = { env: { NODE_ENV: "production" } });
})();
function l() {
  var d, i;
  const e = ((d = window.openai) == null ? void 0 : d.toolOutput) || {}, o = ((i = window.openai) == null ? void 0 : i.theme) || "light", s = e.trend || "stable", t = e.change || 0, r = e.period || "month";
  return /* @__PURE__ */ n.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ n.jsxs("h2", { style: { marginTop: 0 }, children: [
      "Spending Trends (",
      r,
      ")"
    ] }),
    /* @__PURE__ */ n.jsxs("div", { style: {
      padding: "12px",
      background: o === "dark" ? "#303030" : "#F3F3F3",
      borderRadius: "8px"
    }, children: [
      /* @__PURE__ */ n.jsxs("div", { children: [
        "Trend: ",
        /* @__PURE__ */ n.jsx("strong", { children: s })
      ] }),
      /* @__PURE__ */ n.jsxs("div", { children: [
        "Change: ",
        t > 0 ? "+" : "",
        t,
        "%"
      ] })
    ] }),
    /* @__PURE__ */ n.jsx("p", { style: { fontSize: "14px", color: o === "dark" ? "#AFAFAF" : "#5D5D5D" }, children: "Chart visualization will be added when Apps SDK UI Chart components are available." })
  ] });
}
if (typeof window < "u") {
  const e = document.getElementById("root");
  e && a(e).render(/* @__PURE__ */ n.jsx(l, {}));
}
export {
  l as default
};
