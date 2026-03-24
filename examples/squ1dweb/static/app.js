const getFormData = form => {
  const data = new URLSearchParams(new FormData(form));
  return data.toString();
};

const safeJson = text => {
  try {
    return JSON.parse(text);
  } catch (_) {
    return null;
  }
};

const renderResult = (id, status, body) => {
  const node = document.getElementById(id);
  const maybeJson = safeJson(body);
  if (maybeJson !== null) {
    node.textContent = `${status}\n${JSON.stringify(maybeJson, null, 2)}`;
  } else {
    node.textContent = `${status}\n${body}`;
  }
};

let authToken = "";

const callApi = async (path, body) => {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body
  });
  const text = await response.text();
  return { status: response.status, text };
};

document.getElementById("registerForm").addEventListener("submit", async ev => {
  ev.preventDefault();
  const body = getFormData(ev.target);
  const data = await callApi("/api/register", body);
  renderResult("registerResult", data.status, data.text);
});

document.getElementById("loginForm").addEventListener("submit", async ev => {
  ev.preventDefault();
  const body = getFormData(ev.target);
  const data = await callApi("/api/login", body);
  const parsed = safeJson(data.text);
  if (parsed && parsed.token) {
    authToken = parsed.token;
  }
  renderResult("loginResult", data.status, data.text);
});

document.getElementById("dashBtn").addEventListener("click", async () => {
  const headers = {};
  if (authToken) {
    headers.Authorization = `Bearer ${authToken}`;
  } else {
    headers.Authorization = "Bearer dummy-token";
  }
  const resp = await fetch("/api/dashboard", { headers });
  const text = await resp.text();
  renderResult("dashResult", resp.status, text);
});
