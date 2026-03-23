const getFormData = form => {
  const data = new URLSearchParams(new FormData(form));
  return data.toString();
};

const renderResult = (id, text) => {
  document.getElementById(id).textContent = text;
};

const callApi = async (path, body) => {
  const response = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body,
  });
  const text = await response.text();
  return { status: response.status, text };
};

document.getElementById('registerForm').addEventListener('submit', async ev => {
  ev.preventDefault();
  const body = getFormData(ev.target);
  const data = await callApi('/api/register', body);
  renderResult('registerResult', `${data.status}: ${data.text}`);
});

document.getElementById('loginForm').addEventListener('submit', async ev => {
  ev.preventDefault();
  const body = getFormData(ev.target);
  const data = await callApi('/api/login', body);
  renderResult('loginResult', `${data.status}: ${data.text}`);
});

document.getElementById('dashBtn').addEventListener('click', async () => {
  const resp = await fetch('/api/dashboard', {
    headers: { 'Authorization': 'Bearer dummy-token' }
  });
  const text = await resp.text();
  renderResult('dashResult', `${resp.status}: ${text}`);
});
