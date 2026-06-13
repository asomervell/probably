import { c as p, j as n } from "./client-BV8LgGti.js";
import { S as u } from "./StatCard-CN26b5Ll.js";
(function() {
  var o = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  o && typeof o.process > "u" && (o.process = { env: { NODE_ENV: "production" } });
})();
function c() {
  var i, d;
  const o = ((i = window.openai) == null ? void 0 : i.toolOutput) || {}, t = ((d = window.openai) == null ? void 0 : d.theme) || "light";
  typeof window < "u" && window.console && (console.log("[SpendingSummary] toolOutput:", o), console.log("[SpendingSummary] full window.openai:", window.openai));
  const s = o.total || "0", a = o.period || "month", e = o.by_category || [];
  return !o.total && e.length === 0 ? /* @__PURE__ */ n.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif", color: "#ff6b6b" }, children: [
    /* @__PURE__ */ n.jsx("h2", { style: { marginTop: 0 }, children: "Spending Summary" }),
    /* @__PURE__ */ n.jsx("p", { children: "No data available. Check browser console for details." }),
    /* @__PURE__ */ n.jsx("pre", { style: { fontSize: "12px", background: "#1a1a1a", padding: "8px", borderRadius: "4px", overflow: "auto" }, children: JSON.stringify({ toolOutput: o, fullOpenAI: window.openai }, null, 2) })
  ] }) : /* @__PURE__ */ n.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ n.jsxs("h2", { style: { marginTop: 0 }, children: [
      "Spending Summary (",
      a,
      ")"
    ] }),
    /* @__PURE__ */ n.jsx(
      u,
      {
        value: s,
        label: "Total Spending",
        trend: null
      }
    ),
    e.length > 0 && /* @__PURE__ */ n.jsxs("div", { style: { marginTop: "16px" }, children: [
      /* @__PURE__ */ n.jsx("h3", { children: "By Category" }),
      e.map((r, l) => /* @__PURE__ */ n.jsxs("div", { style: { marginBottom: "8px", padding: "8px", background: t === "dark" ? "#303030" : "#F3F3F3" }, children: [
        /* @__PURE__ */ n.jsx("strong", { children: r.category }),
        ": ",
        r.amount
      ] }, l))
    ] })
  ] });
}
if (typeof window < "u") {
  const o = document.getElementById("root");
  o && p(o).render(/* @__PURE__ */ n.jsx(c, {}));
}
export {
  c as default
};
