import { c as r, j as o } from "./client-BV8LgGti.js";
(function() {
  var e = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  e && typeof e.process > "u" && (e.process = { env: { NODE_ENV: "production" } });
})();
function a() {
  var i, s;
  const e = ((i = window.openai) == null ? void 0 : i.toolOutput) || {}, n = ((s = window.openai) == null ? void 0 : s.theme) || "light", d = e.answer || "No answer available", t = e.question || "";
  return /* @__PURE__ */ o.jsxs("div", { style: { padding: "16px", fontFamily: "system-ui, sans-serif" }, children: [
    t && /* @__PURE__ */ o.jsxs("h3", { style: { marginTop: 0 }, children: [
      "Q: ",
      t
    ] }),
    /* @__PURE__ */ o.jsx("div", { style: {
      padding: "12px",
      background: n === "dark" ? "#303030" : "#F3F3F3",
      borderRadius: "8px",
      whiteSpace: "pre-wrap"
    }, children: d })
  ] });
}
if (typeof window < "u") {
  const e = document.getElementById("root");
  e && r(e).render(/* @__PURE__ */ o.jsx(a, {}));
}
export {
  a as default
};
