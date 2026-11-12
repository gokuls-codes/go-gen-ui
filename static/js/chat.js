const sendBtn = document.getElementById("send-btn");

document.addEventListener("htmx:beforeRequest", (e) => {
  if (e.target.id === "chat-form") sendBtn.disabled = true;
});

document.addEventListener("htmx:afterRequest", (e) => {
  if (e.detail.pathInfo?.requestPath.startsWith("/chat/reply")) {
    sendBtn.disabled = false;
  }
});

document.addEventListener("htmx:responseError", () => {
  sendBtn.disabled = false;
});
