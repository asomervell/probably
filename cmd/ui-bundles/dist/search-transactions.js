import { c as h, j as n } from "./client-BV8LgGti.js";
(function() {
  var o = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  o && typeof o.process > "u" && (o.process = { env: { NODE_ENV: "production" } });
})();
function u() {
  var d, r, a;
  const o = ((d = window.openai) == null ? void 0 : d.toolOutput) || {}, s = ((r = window.openai) == null ? void 0 : r.toolResponseMetadata) || {}, t = ((a = window.openai) == null ? void 0 : a.theme) || "light", l = o.count || 0, i = o.query || "", e = s.transactions || [];
  return /* @__PURE__ */ n.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    /* @__PURE__ */ n.jsx("h2", { style: { marginTop: 0 }, children: "Transaction Search" }),
    i && /* @__PURE__ */ n.jsxs("p", { children: [
      "Query: ",
      /* @__PURE__ */ n.jsx("strong", { children: i })
    ] }),
    /* @__PURE__ */ n.jsxs("p", { children: [
      "Found: ",
      /* @__PURE__ */ n.jsx("strong", { children: l }),
      " transactions"
    ] }),
    e.length > 0 && /* @__PURE__ */ n.jsxs("div", { style: { marginTop: "16px" }, children: [
      e.slice(0, 10).map((c, p) => /* @__PURE__ */ n.jsxs("div", { style: {
        marginBottom: "8px",
        padding: "12px",
        background: t === "dark" ? "#303030" : "#F3F3F3",
        borderRadius: "8px"
      }, children: [
        /* @__PURE__ */ n.jsx("div", { children: /* @__PURE__ */ n.jsx("strong", { children: c.description || "No description" }) }),
        /* @__PURE__ */ n.jsx("div", { style: { fontSize: "14px", color: t === "dark" ? "#AFAFAF" : "#5D5D5D" }, children: c.date || "No date" })
      ] }, p)),
      e.length > 10 && /* @__PURE__ */ n.jsxs("p", { style: { fontSize: "14px", color: t === "dark" ? "#AFAFAF" : "#5D5D5D" }, children: [
        "... and ",
        e.length - 10,
        " more"
      ] })
    ] })
  ] });
}
if (typeof window < "u") {
  const o = document.getElementById("root");
  o && h(o).render(/* @__PURE__ */ n.jsx(u, {}));
}
export {
  u as default
};
