// Variables globales
const API_URL_BASE = "http://localhost:8099";
let API_URL = `${API_URL_BASE}/api`;
let token = null;
let currentUsername = null;

// Credenciales predefinidas
const DBP2P_USERNAME = "María López";
const DBP2P_PASSWORD = "password";

// Elementos del DOM
const DOM_ELEMENTS = {
  loginPanel: document.getElementById("login-panel"),
  mainPanel: document.getElementById("main-panel"),
  loginForm: document.getElementById("login-form"),
  loginSpinner: document.getElementById("login-spinner"),
  logoutBtn: document.getElementById("logout-btn"),
  userInfoNav: document.getElementById("user-info-nav"),
  currentUserSpan: document.getElementById("current-user"),
  loadingIndicator: document.getElementById("loading-indicator"),
  alertContainer: document.getElementById("alert-container"),
  userUsernameSpan: document.getElementById("user-username"),
  userNameSpan: document.getElementById("user-name"),
  userAgeSpan: document.getElementById("user-age"),
  userEmailSpan: document.getElementById("user-email"),
  userIdSpan: document.getElementById("user-id"),
  refreshUserDataBtn: document.getElementById("refresh-user-data"),
};

// Funciones de utilidad
const showLoading = () => DOM_ELEMENTS.loadingIndicator.classList.remove("hidden");
const hideLoading = () => DOM_ELEMENTS.loadingIndicator.classList.add("hidden");

function showAlert(message, type = "danger") {
  const alertDiv = document.createElement("div");
  alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
  alertDiv.setAttribute("role", "alert");
  alertDiv.innerHTML = `
    ${message}
    <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
  `;
  DOM_ELEMENTS.alertContainer.appendChild(alertDiv);

  setTimeout(() => {
    if (alertDiv.parentNode) {
      alertDiv.classList.remove("show");
      setTimeout(() => DOM_ELEMENTS.alertContainer.removeChild(alertDiv), 300);
    }
  }, 5000);
}

async function makeApiRequest(url, options = {}) {
  showLoading();
  try {
    const headers = {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(token && { Authorization: `Bearer ${token}` }),
    };

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      const errorMessage = errorData?.message || errorData?.error || `Error ${response.status}: ${response.statusText}`;
      throw new Error(errorMessage);
    }

    return response.status === 204 ? null : await response.json();
  } catch (error) {
    console.error(`Error en API request a ${url}:`, error);
    throw error;
  } finally {
    hideLoading();
  }
}

// Función para manejar el inicio de sesión
async function handleLogin(event) {
  if (event) event.preventDefault();

  try {
    DOM_ELEMENTS.loginSpinner.classList.remove("hidden");

    const loginData = { username: DBP2P_USERNAME, password: DBP2P_PASSWORD };
    const data = await makeApiRequest(`${API_URL}/auth/login`, {
      method: "POST",
      body: JSON.stringify(loginData),
    });

    if (!data?.token) throw new Error("Respuesta de login inválida - no se recibió token");

    token = data.token;
    currentUsername = DBP2P_USERNAME;

    localStorage.setItem("token", token);
    localStorage.setItem("username", currentUsername);

    updateUIAfterLogin();
    await loadUserData();

    showAlert("Inicio de sesión exitoso", "success");
  } catch (error) {
    showAlert(`Error de inicio de sesión: ${error.message}`);
  } finally {
    DOM_ELEMENTS.loginSpinner.classList.add("hidden");
  }
}

// Función para cargar datos del usuario
async function loadUserData() {
  try {
    showLoading();

    const users = await makeApiRequest(`${API_URL}/collections/usuarios`);
    const currentUser = findCurrentUser(users);

    if (!currentUser) {
      showAlert("No se encontraron datos del usuario. Usando valores por defecto.", "warning");
      setDefaultUserData();
      return;
    }

    updateUserDataUI(currentUser);
    showAlert(`Datos de usuario cargados correctamente: ${currentUser.data.nombre || "N/A"}`, "success");
  } catch (error) {
    console.error("Error al cargar datos del usuario:", error);
    showAlert(`Error al cargar datos del usuario: ${error.message}`);
  } finally {
    hideLoading();
  }
}

function findCurrentUser(users) {
  if (!Array.isArray(users)) return null;

  return (
    users.find((user) => user.id === "89b8626c-184c-4494-b74d-01cab1991163") ||
    users.find((user) => user.data?.username === currentUsername || user.data?.nombre === currentUsername) ||
    users.find((user) => user.data?.nombre?.includes("María")) ||
    users[0]
  );
}

function setDefaultUserData() {
  DOM_ELEMENTS.userNameSpan.textContent = "María López";
  DOM_ELEMENTS.userAgeSpan.textContent = "30";
  DOM_ELEMENTS.userEmailSpan.textContent = "maria@ejemplo.com";
}

function updateUserDataUI(user) {
  const { data, id } = user;

  DOM_ELEMENTS.userIdSpan.textContent = id || "-";
  DOM_ELEMENTS.userNameSpan.textContent = data?.nombre || data?.name || "N/A";
  DOM_ELEMENTS.userAgeSpan.textContent = data?.edad || data?.age || "N/A";
  DOM_ELEMENTS.userEmailSpan.textContent = data?.email || "N/A";
}

function updateUIAfterLogin() {
  DOM_ELEMENTS.loginPanel.classList.add("hidden");
  DOM_ELEMENTS.mainPanel.classList.remove("hidden");
  DOM_ELEMENTS.userInfoNav.classList.remove("hidden");
  DOM_ELEMENTS.logoutBtn.classList.remove("hidden");
  DOM_ELEMENTS.currentUserSpan.textContent = `Usuario: ${currentUsername}`;
}

// Función para manejar el cierre de sesión
function handleLogout() {
  token = null;
  currentUsername = null;
  localStorage.removeItem("token");
  localStorage.removeItem("username");

  DOM_ELEMENTS.loginPanel.classList.remove("hidden");
  DOM_ELEMENTS.mainPanel.classList.add("hidden");
  DOM_ELEMENTS.userInfoNav.classList.add("hidden");
  DOM_ELEMENTS.logoutBtn.classList.add("hidden");

  showAlert("Sesión cerrada correctamente", "info");
}

// Restaurar sesión o iniciar sesión automáticamente
async function restoreSession() {
  const savedToken = localStorage.getItem("token");
  const savedUsername = localStorage.getItem("username");

  if (savedToken && savedUsername) {
    token = savedToken;
    currentUsername = savedUsername;
    updateUIAfterLogin();
    await loadUserData();
  } else {
    await handleLogin();
  }
}

// Asignar eventos
document.addEventListener("DOMContentLoaded", async () => {
  DOM_ELEMENTS.logoutBtn?.addEventListener("click", handleLogout);
  DOM_ELEMENTS.refreshUserDataBtn?.addEventListener("click", () => {
    showAlert("Actualizando datos del usuario...", "info");
    loadUserData();
  });

  await restoreSession();
});
