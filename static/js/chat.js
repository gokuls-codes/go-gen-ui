const sendBtn = document.getElementById("send-btn");

document.addEventListener("htmx:beforeRequest", (e) => {
  if (e.target.id === "chat-form") sendBtn.disabled = true;
});

document.addEventListener("htmx:sseClose", () => {
  sendBtn.disabled = false;
});

document.addEventListener("htmx:responseError", () => {
  sendBtn.disabled = false;
});
