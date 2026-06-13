import { c as p, j as o } from "./client-BV8LgGti.js";
import { S as c } from "./StatCard-CN26b5Ll.js";
(function() {
  var e = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  e && typeof e.process > "u" && (e.process = { env: { NODE_ENV: "production" } });
})();
function g() {
  var r, d;
  const e = ((r = window.openai) == null ? void 0 : r.toolOutput) || {}, t = ((d = window.openai) == null ? void 0 : d.theme) || "light";
  typeof window < "u" && window.console && (console.log("[SpendingSummary] toolOutput:", e), console.log("[SpendingSummary] full window.openai:", window.openai));
  const n = e.total || "0", s = e.period || "month", i = e.by_category || [];
  return !e.total && i.length === 0 ? /* @__PURE__ */ o.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif", color: "#ff6b6b" }, children: [
    /* @__PURE__ */ o.jsx("h2", { style: { marginTop: 0 }, children: "Spending Summary" }),
    /* @__PURE__ */ o.jsx("p", { children: "No data available. Check browser console for details." }),
    /* @__PURE__ */ o.jsx("pre", { style: { fontSize: "12px", background: "#1a1a1a", padding: "8px", borderRadius: "4px", overflow: "auto" }, children: JSON.stringify({ toolOutput: e, fullOpenAI: window.openai }, null, 2) })
  ] }) : /* @__PURE__ */ o.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ o.jsxs("h2", { style: { marginTop: 0 }, children: [
      "Spending Summary (",
      s,
      ")"
    ] }),
    /* @__PURE__ */ o.jsx(
      c,
      {
        value: n,
        label: "Total Spending",
        trend: null
      }
    ),
    i.length > 0 && /* @__PURE__ */ o.jsxs("div", { style: { marginTop: "16px" }, children: [
      /* @__PURE__ */ o.jsx("h3", { children: "By Category" }),
      i.map((a, l) => /* @__PURE__ */ o.jsxs("div", { style: { marginBottom: "8px", padding: "8px", background: t === "dark" ? "#303030" : "#F3F3F3" }, children: [
        /* @__PURE__ */ o.jsx("strong", { children: a.category }),
        ": ",
        a.amount
      ] }, l))
    ] })
  ] });
}
if (typeof window < "u") {
  const e = () => {
    const t = document.getElementById("root");
    if (t)
      try {
        p(t).render(/* @__PURE__ */ o.jsx(g, {}));
      } catch (n) {
        console.error("[SpendingSummary] Error rendering:", n), t.innerHTML = `<div style="padding: 16px; color: #ff6b6b;">
          <h2>Error Loading Widget</h2>
          <p>${n instanceof Error ? n.message : String(n)}</p>
        </div>`;
      }
    else
      console.error("[SpendingSummary] Root element not found");
  };
  document.readyState === "loading" ? document.addEventListener("DOMContentLoaded", e) : e();
}
export {
  g as default
};
