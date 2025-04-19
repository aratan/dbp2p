// Configuración
// Detectar si estamos en HTTPS para usar URLs seguras por defecto
const isSecure = window.location.protocol === "https:";
let API_URL = `${isSecure ? "https" : "http"}://localhost:8099/api`; // Valor por defecto, se actualizará desde el formulario
let WS_URL = `${isSecure ? "wss" : "ws"}://localhost:8100/ws`; // Valor por defecto, se actualizará desde el formulario
let token = null;
let ws = null;
let currentUsername = null; // Almacena el nombre de usuario logueado

/**
 * Verifica si un documento contiene datos binarios
 * @param {Object} doc - Documento a verificar
 * @returns {boolean} - true si el documento contiene datos binarios
 */
function isBinaryDocument(doc) {
  // Verificar si el documento tiene datos
  if (!doc || !doc.data) return false;

  // Verificar si tiene campos característicos de archivos binarios
  const hasBinaryField = !!doc.data.binary;
  const isMultipart = doc.data.isMultipart === true;
  const hasFileMetadata = !!(doc.data.mimetype || doc.data.filename);
  const hasFileSize = !!doc.data.size;

  console.log(`Verificando documento ${doc.id}:`, {
    hasBinaryField,
    isMultipart,
    hasFileMetadata,
    hasFileSize,
    data: doc.data,
  });

  // Un documento es binario si tiene el campo binary o es multipart
  // o si tiene metadatos de archivo (mimetype o filename) y tamaño
  return hasBinaryField || isMultipart || (hasFileMetadata && hasFileSize);
}

/**
 * Obtiene el icono de Bootstrap Icons para un tipo MIME.
 * @param {string} mimeType - Tipo MIME del archivo.
 * @returns {string} - Clase de icono de Bootstrap Icons.
 */
function getFileIcon(mimeType) {
  if (!mimeType) return "bi-file";

  const type = mimeType.split("/")[0];
  const subtype = mimeType.split("/")[1];

  switch (type) {
    case "image":
      return "bi-file-image";
    case "audio":
      return "bi-file-music";
    case "video":
      return "bi-file-play";
    case "text":
      return "bi-file-text";
    case "application":
      if (subtype === "pdf") return "bi-file-pdf";
      if (subtype.includes("word") || subtype === "msword")
        return "bi-file-word";
      if (subtype.includes("excel") || subtype.includes("spreadsheet"))
        return "bi-file-excel";
      if (subtype.includes("powerpoint") || subtype.includes("presentation"))
        return "bi-file-ppt";
      if (
        subtype.includes("zip") ||
        subtype.includes("compressed") ||
        subtype.includes("archive")
      )
        return "bi-file-zip";
      if (subtype.includes("json")) return "bi-file-code";
      return "bi-file-binary";
    default:
      return "bi-file";
  }
}

/**
 * Formatea un tamaño de archivo en bytes a una representación legible.
 * @param {number} bytes - Tamaño en bytes.
 * @returns {string} - Tamaño formateado.
 */
function formatFileSize(bytes) {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

// Sistema de caché
const cache = {
  data: {}, // Almacena los datos en caché
  timeouts: {}, // Almacena los timeouts para limpiar la caché
  defaultTTL: 60000, // Tiempo de vida por defecto (60 segundos)

  /**
   * Guarda un valor en la caché
   * @param {string} key - Clave para identificar el valor
   * @param {any} value - Valor a almacenar
   * @param {number} [ttl=cache.defaultTTL] - Tiempo de vida en milisegundos
   */
  set(key, value, ttl = this.defaultTTL) {
    // Limpiar timeout anterior si existe
    if (this.timeouts[key]) {
      clearTimeout(this.timeouts[key]);
    }

    // Guardar valor
    this.data[key] = {
      value,
      timestamp: Date.now(),
    };

    // Configurar timeout para limpiar
    this.timeouts[key] = setTimeout(() => {
      delete this.data[key];
      delete this.timeouts[key];
      console.log(`Caché: Expiró la clave ${key}`);
    }, ttl);

    console.log(`Caché: Guardado ${key} (TTL: ${ttl}ms)`);
  },

  /**
   * Obtiene un valor de la caché
   * @param {string} key - Clave del valor a obtener
   * @returns {any|null} El valor almacenado o null si no existe o ha expirado
   */
  get(key) {
    const entry = this.data[key];
    if (!entry) {
      console.log(`Caché: Miss para ${key}`);
      return null;
    }

    console.log(
      `Caché: Hit para ${key} (edad: ${Date.now() - entry.timestamp}ms)`
    );
    return entry.value;
  },

  /**
   * Elimina un valor de la caché
   * @param {string} key - Clave del valor a eliminar
   */
  delete(key) {
    if (this.timeouts[key]) {
      clearTimeout(this.timeouts[key]);
      delete this.timeouts[key];
    }

    delete this.data[key];
    console.log(`Caché: Eliminado ${key}`);
  },

  /**
   * Elimina todos los valores que coincidan con un patrón
   * @param {RegExp} pattern - Patrón para comparar con las claves
   */
  invalidatePattern(pattern) {
    Object.keys(this.data).forEach((key) => {
      if (pattern.test(key)) {
        this.delete(key);
      }
    });
  },

  /**
   * Limpia toda la caché
   */
  clear() {
    Object.keys(this.timeouts).forEach((key) => {
      clearTimeout(this.timeouts[key]);
    });

    this.data = {};
    this.timeouts = {};
    console.log("Caché: Limpiada completamente");
  },
};

// Referencias a Elementos DOM (se asignarán en DOMContentLoaded)
let loadingIndicator,
  loginSection,
  dashboard,
  userInfo,
  currentUserSpan,
  usernameInput,
  passwordInput,
  collectionInput,
  documentDataInput,
  documentsList,
  wsCollectionInput,
  realTimeUpdatesList,
  alertContainer,
  tabsContainer,
  tabButtons,
  backupsList,
  currentWsSubscriptionSpan,
  connectionStatus,
  logoutBtn,
  loginSpinner,
  mainPanel;

// --- Utilidades ---

const showLoading = () => {
  if (loadingIndicator) loadingIndicator.style.display = "block";
};
const hideLoading = () => {
  if (loadingIndicator) loadingIndicator.style.display = "none";
};

/**
 * Muestra una alerta (tipo Bootstrap) en el contenedor de alertas.
 * @param {string} message - El mensaje a mostrar.
 * @param {string} [type='danger'] - El tipo de alerta ('success', 'info', 'warning', 'danger').
 */
const showAlert = (message, type = "danger") => {
  if (!alertContainer) {
    console.error("Contenedor de alertas (#alertContainer) no encontrado.");
    return;
  }
  const alertDiv = document.createElement("div");
  // Clases de Bootstrap para alertas con botón de cierre
  alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
  alertDiv.setAttribute("role", "alert");
  alertDiv.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `;
  // Inserta la alerta al principio del contenedor
  alertContainer.insertBefore(alertDiv, alertContainer.firstChild);
};

/**
 * Valida y parsea una cadena JSON.
 * @param {string} str - La cadena a validar.
 * @returns {object} El objeto parseado.
 * @throws {Error} Si la cadena está vacía o no es JSON válido.
 */
const validateJSON = (str) => {
  if (!str || !str.trim()) {
    throw new Error("El campo de datos JSON no puede estar vacío.");
  }
  try {
    return JSON.parse(str);
  } catch (e) {
    console.error("Error al parsear JSON:", e);
    throw new Error(`JSON inválido: ${e.message}`);
  }
};

/**
 * Realiza una petición a la API, manejando el token, loading y errores básicos.
 * Incluye sistema de reintentos para errores temporales de red y caché para peticiones GET.
 * @param {string} url - URL completa del endpoint.
 * @param {object} [options={}] - Opciones para fetch (method, body, headers adicionales).
 * @param {number} [retryCount=0] - Número de reintentos realizados (uso interno).
 * @param {number} [maxRetries=3] - Número máximo de reintentos.
 * @param {number} [retryDelay=1000] - Tiempo de espera entre reintentos (ms).
 * @param {boolean} [useCache=true] - Si se debe usar la caché para peticiones GET.
 * @param {number} [cacheTTL=60000] - Tiempo de vida de la caché en milisegundos (60 segundos por defecto).
 * @returns {Promise<any>} La respuesta parseada (JSON o texto).
 * @throws {Error} Si la petición falla o la respuesta no es OK después de todos los reintentos.
 */
async function makeApiRequest(
  url,
  options = {},
  retryCount = 0,
  maxRetries = 3,
  retryDelay = 1000,
  useCache = true,
  cacheTTL = 60000
) {
  const method = options.method || "GET";
  const isGetRequest = method === "GET";
  const cacheKey = `${url}:${JSON.stringify(options.headers || {})}`;

  // Usar caché solo para peticiones GET si está habilitado
  if (isGetRequest && useCache) {
    const cachedData = cache.get(cacheKey);
    if (cachedData) {
      console.log(`Usando datos en caché para: ${url}`);
      return cachedData;
    }
  }

  showLoading();
  try {
    const defaultHeaders = {
      "Content-Type": "application/json",
      Accept: "application/json", // Indica que esperamos JSON
    };
    if (token) {
      defaultHeaders["Authorization"] = `Bearer ${token}`;
    }

    // Imprimir para depuración
    console.log(
      `Realizando petición a ${url}${
        retryCount > 0 ? ` (reintento ${retryCount}/${maxRetries})` : ""
      }`
    );
    console.log("Método:", method);
    console.log("Cabeceras:", { ...defaultHeaders, ...options.headers });
    if (options.body) {
      console.log("Cuerpo:", options.body);
    }

    // Agregar un timeout a la petición
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 30000); // 30 segundos de timeout

    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          ...defaultHeaders,
          ...options.headers, // Permite sobrescribir cabeceras
        },
        signal: controller.signal,
      });

      // Limpiar el timeout
      clearTimeout(timeoutId);

      console.log(
        `Respuesta recibida: ${response.status} ${response.statusText}`
      );
      console.log(
        "Cabeceras de respuesta:",
        Object.fromEntries([...response.headers.entries()])
      );

      // Intenta obtener un mensaje de error del cuerpo si la respuesta no es OK
      if (!response.ok) {
        let errorMessage = `Error ${response.status}: ${response.statusText}`;
        try {
          const errorData = await response.json();
          console.log("Datos de error:", errorData);
          errorMessage = errorData.message || errorData.error || errorMessage;
        } catch (jsonError) {
          // Ignora si el cuerpo no es JSON o está vacío
          console.log("No se pudo parsear el cuerpo del error como JSON");
        }
        throw new Error(errorMessage);
      }

      // Maneja respuestas sin contenido (ej. DELETE 204)
      if (response.status === 204) {
        console.log("Respuesta sin contenido (204)");
        return null; // O un objeto vacío {}, dependiendo de cómo quieras manejarlo
      }

      // Intenta parsear como JSON si el content-type lo indica
      const contentType = response.headers.get("content-type");
      console.log("Content-Type de respuesta:", contentType);

      if (contentType && contentType.includes("application/json")) {
        const jsonData = await response.json();
        console.log("Datos JSON recibidos:", jsonData);

        // Guardar en caché si es una petición GET y está habilitada la caché
        if (isGetRequest && useCache) {
          cache.set(cacheKey, jsonData, cacheTTL);
        }

        return jsonData;
      } else {
        // Devuelve texto si no es JSON (o maneja otros tipos si es necesario)
        const textData = await response.text();
        console.log("Datos de texto recibidos:", textData);

        // Guardar en caché si es una petición GET y está habilitada la caché
        if (isGetRequest && useCache) {
          cache.set(cacheKey, textData, cacheTTL);
        }

        return textData;
      }
    } catch (fetchError) {
      // Limpiar el timeout
      clearTimeout(timeoutId);

      // Determinar si el error es candidato para reintento
      const isRetryableError =
        fetchError.name === "AbortError" || // Timeout
        fetchError.message.includes("Failed to fetch") || // Error de red
        fetchError.message.includes("NetworkError") || // Error de red
        (fetchError.message.includes("fetch") &&
          fetchError.message.includes("network")); // Otros errores de red

      // Si es un error que se puede reintentar y no hemos alcanzado el límite de reintentos
      if (isRetryableError && retryCount < maxRetries) {
        console.log(
          `Error retryable detectado: ${fetchError.message}. Reintentando (${
            retryCount + 1
          }/${maxRetries})...`
        );

        // Esperar antes de reintentar (tiempo exponencial: 1s, 2s, 4s, ...)
        const delay = retryDelay * Math.pow(2, retryCount);
        await new Promise((resolve) => setTimeout(resolve, delay));

        // Reintentar la petición con un contador incrementado
        return makeApiRequest(
          url,
          options,
          retryCount + 1,
          maxRetries,
          retryDelay,
          useCache,
          cacheTTL
        );
      }

      // Si es un error de timeout y ya no hay más reintentos
      if (fetchError.name === "AbortError") {
        throw new Error(
          `La petición ha excedido el tiempo de espera después de ${retryCount} reintentos`
        );
      }

      // Reenviar otros errores o errores después de agotar los reintentos
      throw fetchError;
    }
  } catch (error) {
    // Lanza el error para que sea capturado por la función que llama
    console.error(
      `Error en API request a ${url} después de ${retryCount} reintentos:`,
      error
    );
    throw error; // Asegúrate de que el error se propague
  } finally {
    // Solo ocultar el indicador de carga si no estamos en un reintento
    // o si es el último reintento fallido
    if (retryCount === 0 || retryCount >= maxRetries) {
      hideLoading();
    }
  }
}

// --- Gestión de Pestañas ---

/**
 * Muestra la pestaña especificada y oculta las demás.
 * @param {string} tabName - El nombre de la pestaña (ej. 'documents', 'websocket', 'backups').
 */
function showTab(tabName) {
  // Oculta todos los contenidos de pestañas
  document
    .querySelectorAll(".tab-content")
    .forEach((tab) => tab.classList.remove("active"));
  // Desactiva todos los botones de pestañas
  tabButtons.forEach((btn) => btn.classList.remove("active"));

  // Muestra el contenido de la pestaña objetivo
  const targetTabContent = document.getElementById(tabName);
  if (targetTabContent) {
    targetTabContent.classList.add("show", "active");
  } else {
    console.error(`Contenido de pestaña no encontrado para: ${tabName}`);
  }

  // Activa el botón correspondiente
  const targetButton = document.getElementById(`${tabName}-tab`);
  if (targetButton) {
    targetButton.classList.add("active");
  } else {
    console.error(`Botón de pestaña no encontrado para: ${tabName}-tab`);
  }

  // Carga datos relevantes al cambiar a ciertas pestañas
  switch (tabName) {
    case "collections":
      refreshDocuments();
      break;
    case "users":
      loadUsers();
      break;
    case "backups":
      listBackups();
      break;
    case "live":
      // Podrías querer limpiar la lista de updates o mostrar estado de conexión
      break;
  }
}

// --- Autenticación ---

/**
 * Maneja el envío del formulario de login.
 * @param {Event} [event] - El evento de submit (opcional).
 */
async function handleLogin(event) {
  if (event) event.preventDefault(); // Previene recarga de página si es un evento submit
  const username = usernameInput.value.trim();
  const password = passwordInput.value;

  // Actualizar las URLs desde el formulario
  const serverUrlInput = document.getElementById("server-url");
  const wsUrlInput = document.getElementById("ws-url");

  if (serverUrlInput && serverUrlInput.value) {
    let serverUrl = serverUrlInput.value.trim();

    // Asegurarse de que la URL tenga el protocolo correcto
    if (!serverUrl.startsWith("http://") && !serverUrl.startsWith("https://")) {
      // Si no tiene protocolo, usar el protocolo actual de la página
      serverUrl = `${isSecure ? "https" : "http"}://${serverUrl}`;
    } else if (isSecure && serverUrl.startsWith("http://")) {
      // Advertir sobre contenido mixto
      showAlert(
        "Advertencia: Estás usando HTTP en una página HTTPS. Esto puede causar problemas de contenido mixto.",
        "warning"
      );
    }

    API_URL = serverUrl;
    if (!API_URL.endsWith("/api")) {
      API_URL = API_URL + "/api";
    }
  }

  if (wsUrlInput && wsUrlInput.value) {
    let wsUrl = wsUrlInput.value.trim();

    // Asegurarse de que la URL tenga el protocolo correcto
    if (!wsUrl.startsWith("ws://") && !wsUrl.startsWith("wss://")) {
      // Si no tiene protocolo, usar el protocolo adecuado según si la página es segura
      wsUrl = `${isSecure ? "wss" : "ws"}://${wsUrl}`;
    } else if (isSecure && wsUrl.startsWith("ws://")) {
      // Advertir sobre contenido mixto
      showAlert(
        "Advertencia: Estás usando WS en una página HTTPS. Esto puede causar problemas de contenido mixto.",
        "warning"
      );
    }

    WS_URL = wsUrl;
    if (!WS_URL.endsWith("/ws")) {
      WS_URL = WS_URL + "/ws";
    }
  }

  console.log("Usando API URL:", API_URL);
  console.log("Usando WS URL:", WS_URL);

  // Validación mejorada de los campos
  if (!username) {
    showAlert("Por favor ingrese un nombre de usuario.");
    usernameInput.classList.add("is-invalid");
    usernameInput.focus();
    return;
  } else {
    usernameInput.classList.remove("is-invalid");
    usernameInput.classList.add("is-valid");
  }

  if (!password) {
    showAlert("Por favor ingrese una contraseña.");
    passwordInput.classList.add("is-invalid");
    passwordInput.focus();
    return;
  } else {
    passwordInput.classList.remove("is-invalid");
    passwordInput.classList.add("is-valid");
  }

  // Validar URL del servidor
  if (serverUrlInput && serverUrlInput.value) {
    const serverUrl = serverUrlInput.value.trim();
    const urlPattern = /^(https?:\/\/)?([\w\.-]+)(:\d+)?(\/[\w\.-]*)*\/?$/;

    if (!urlPattern.test(serverUrl)) {
      showAlert(
        "La URL del servidor no es válida. Debe ser una URL correcta (ej: http://localhost:8099)."
      );
      serverUrlInput.classList.add("is-invalid");
      serverUrlInput.focus();
      return;
    } else {
      serverUrlInput.classList.remove("is-invalid");
      serverUrlInput.classList.add("is-valid");
    }
  }

  try {
    // Mostrar spinner de carga
    if (loginSpinner) {
      loginSpinner.style.display = "inline-block";
    }

    console.log(`Intentando iniciar sesión con usuario: ${username}`);

    // Preparar datos para enviar
    const loginData = { username, password };
    console.log("Datos de inicio de sesión:", loginData);

    // Usar fetch directamente para depurar
    const response = await fetch(`${API_URL}/auth/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify(loginData),
    });

    console.log(
      `Respuesta recibida: ${response.status} ${response.statusText}`
    );
    console.log(
      "Cabeceras de respuesta:",
      Object.fromEntries([...response.headers.entries()])
    );

    // Clonar la respuesta para poder leerla múltiples veces
    const responseClone = response.clone();

    if (!response.ok) {
      let errorMessage = `Error ${response.status}: ${response.statusText}`;
      try {
        const errorData = await response.json();
        console.log("Datos de error:", errorData);
        errorMessage = errorData.message || errorData.error || errorMessage;
      } catch (jsonError) {
        console.log("No se pudo parsear el cuerpo del error como JSON");
      }
      throw new Error(errorMessage);
    }

    // Intentar parsear la respuesta como JSON
    let data;
    try {
      // Obtener el texto de la respuesta para depurar
      const responseText = await responseClone.text();
      console.log("Texto de respuesta:", responseText);

      // Intentar parsear el texto como JSON
      if (responseText.trim()) {
        data = JSON.parse(responseText);
        console.log("Respuesta de inicio de sesión parseada:", data);
      } else {
        console.error("Respuesta vacía del servidor");
        throw new Error("El servidor devolvió una respuesta vacía");
      }
    } catch (e) {
      console.error("Error al parsear la respuesta como JSON:", e);
      // Intentar leer la respuesta original como texto para el mensaje de error
      const errorText = await response
        .text()
        .catch(() => "<no se pudo leer el texto>");
      throw new Error(
        `Error al parsear la respuesta: ${e.message}. Texto recibido: ${errorText}`
      );
    }

    if (!data || !data.token) {
      throw new Error(
        `Respuesta de login inválida - no se recibió token. Datos recibidos: ${JSON.stringify(
          data
        )}`
      );
    }

    token = data.token;
    currentUsername = data.username || username; // Usar el username de la respuesta si está disponible
    localStorage.setItem("token", token);
    localStorage.setItem("username", currentUsername); // Guarda también el nombre de usuario

    console.log(`Inicio de sesión exitoso para el usuario: ${username}`);
    console.log(`Token recibido: ${token}`);

    // Actualiza la UI
    if (loginSection) loginSection.style.display = "none";
    if (dashboard) dashboard.style.display = "block";
    if (userInfo) userInfo.style.display = "flex";
    if (currentUserSpan)
      currentUserSpan.textContent = `Usuario: ${currentUsername}`;
    if (mainPanel) mainPanel.classList.remove("hidden");

    // Mostrar el botón de cierre de sesión
    if (logoutBtn) {
      logoutBtn.classList.remove("hidden");
    }

    connectWebSocket(); // Conecta el WebSocket después del login exitoso
    showTab("collections"); // Muestra la pestaña de documentos por defecto

    // Cargar la colección "usuarios" automáticamente
    if (collectionInput) {
      collectionInput.value = "usuarios";
      refreshDocuments();
    }
  } catch (error) {
    console.error("Error detallado de inicio de sesión:", error);

    // Mensaje de error más descriptivo
    let errorMessage = error.message;

    if (
      error.message.includes("tiempo de espera") ||
      error.message.includes("timeout")
    ) {
      errorMessage =
        "El servidor está tardando demasiado en responder. Por favor, verifica que el servidor esté ejecutándose correctamente y que la URL sea correcta.";
    } else if (
      error.message.includes("NetworkError") ||
      error.message.includes("Failed to fetch")
    ) {
      errorMessage =
        "No se pudo conectar con el servidor. Por favor, verifica que el servidor esté ejecutándose y que la URL sea correcta.";
    } else if (error.message.includes("Credenciales inválidas")) {
      errorMessage =
        "Usuario o contraseña incorrectos. Por favor, inténtalo de nuevo.";
    }

    showAlert(`Error de inicio de sesión: ${errorMessage}`);

    // Ocultar el spinner de carga
    if (loginSpinner) {
      loginSpinner.style.display = "none";
    }
  }
}

/**
 * Cierra la sesión del usuario.
 */
function logout() {
  token = null;
  currentUsername = null;
  localStorage.removeItem("token");
  localStorage.removeItem("username");

  // Cierra el WebSocket si está abierto
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.close(1000, "User logged out"); // Código 1000 para cierre normal
  }
  ws = null; // Limpia la instancia

  // Restablece la UI al estado de login
  loginSection.style.display = "block";
  dashboard.style.display = "none";
  userInfo.style.display = "none";
  currentUserSpan.textContent = "";
  usernameInput.value = "";
  passwordInput.value = "";

  // Limpia áreas de datos
  if (documentsList) documentsList.innerHTML = "";
  if (realTimeUpdatesList) realTimeUpdatesList.innerHTML = "";
  if (backupsList) backupsList.innerHTML = "";
  if (collectionInput) collectionInput.value = "";
  if (documentDataInput) documentDataInput.value = "";
  if (wsCollectionInput) wsCollectionInput.value = "";
  if (currentWsSubscriptionSpan)
    currentWsSubscriptionSpan.textContent = "No suscrito";

  showAlert("Sesión cerrada exitosamente.", "info");
}

// --- Operaciones con archivos binarios ---

/**
 * Accede directamente a un archivo binario por su ID
 * @param {string} id - ID del documento
 * @param {boolean} [visualize=false] - Si es true, intenta abrir el archivo en el navegador; si es false, lo descarga
 */
async function accessBinaryById(id, visualize = false) {
  try {
    // Intentar determinar la colección del documento
    const collections = ["usuarios", "archivos", "documentos", "binarios"];
    let foundDoc = null;
    let foundCollection = null;

    showLoading();
    showAlert(`Buscando archivo con ID: ${id}...`, "info");

    // Buscar el documento en las colecciones comunes
    for (const collection of collections) {
      try {
        const doc = await makeApiRequest(
          `${API_URL}/collections/${collection}/${id}`
        );
        if (doc && doc.id) {
          foundDoc = doc;
          foundCollection = collection;
          break;
        }
      } catch (error) {
        console.log(
          `Documento no encontrado en colección ${collection}:`,
          error
        );
      }
    }

    if (!foundDoc) {
      hideLoading();
      showAlert(`No se encontró ningún documento con ID: ${id}`, "danger");
      return;
    }

    showAlert(`Archivo encontrado en colección: ${foundCollection}`, "success");

    // Abrir el archivo
    await openBinaryFile(foundCollection, id, visualize);
  } catch (error) {
    hideLoading();
    showAlert(`Error al acceder al archivo: ${error.message}`, "danger");
  }
}

/**
 * Muestra el modal para subir un archivo binario
 */
function showBinaryModal() {
  // Obtener el modal
  const modalElement = document.getElementById("binary-modal");
  const binaryModal = new bootstrap.Modal(modalElement);

  // Configurar el modal
  document.getElementById("binary-collection").value = collectionInput.value;
  document.getElementById("binary-file").value = "";
  document.getElementById("binary-metadata").value = "{}";
  document.getElementById("binary-public-access").checked = true;
  document.getElementById("binary-encrypted").checked = false;
  document.getElementById("binary-password").value = "";
  document.getElementById("binary-allowed-users").value = "";
  document.getElementById("binary-password-container").style.display = "none";

  // Configurar evento para mostrar/ocultar campo de contraseña
  const encryptedCheckbox = document.getElementById("binary-encrypted");
  encryptedCheckbox.addEventListener("change", function () {
    document.getElementById("binary-password-container").style.display = this
      .checked
      ? "block"
      : "none";
  });

  // Mostrar el modal
  binaryModal.show();
}

/**
 * Valida y sube un archivo binario
 */
async function validateAndUploadBinary() {
  // Obtener los datos del formulario
  const collection = document.getElementById("binary-collection").value.trim();
  const fileInput = document.getElementById("binary-file");
  const metadataStr =
    document.getElementById("binary-metadata").value.trim() || "{}";

  // Validar colección
  if (!collection) {
    document.getElementById("binary-collection-feedback").textContent =
      "Por favor especifique una colección.";
    document.getElementById("binary-collection").classList.add("is-invalid");
    document.getElementById("binary-collection").focus();
    return;
  } else {
    document.getElementById("binary-collection").classList.remove("is-invalid");
    document.getElementById("binary-collection").classList.add("is-valid");
  }

  // Validar archivo
  if (!fileInput.files || fileInput.files.length === 0) {
    document.getElementById("binary-file-feedback").textContent =
      "Por favor seleccione un archivo.";
    document.getElementById("binary-file").classList.add("is-invalid");
    document.getElementById("binary-file").focus();
    return;
  } else {
    document.getElementById("binary-file").classList.remove("is-invalid");
    document.getElementById("binary-file").classList.add("is-valid");
  }

  // Validar metadatos
  let metadata = {};
  try {
    metadata = JSON.parse(metadataStr);
    if (
      typeof metadata !== "object" ||
      metadata === null ||
      Array.isArray(metadata)
    ) {
      throw new Error("Los metadatos deben ser un objeto JSON válido.");
    }
  } catch (error) {
    document.getElementById(
      "binary-metadata-feedback"
    ).textContent = `JSON inválido: ${error.message}`;
    document.getElementById("binary-metadata").classList.add("is-invalid");
    document.getElementById("binary-metadata").focus();
    return;
  }

  // Obtener configuración de permisos
  const isPublic = document.getElementById("binary-public-access").checked;
  const isEncrypted = document.getElementById("binary-encrypted").checked;
  const password = document.getElementById("binary-password").value;
  const allowedUsers = document
    .getElementById("binary-allowed-users")
    .value.trim();

  // Validar contraseña si el cifrado está habilitado
  if (isEncrypted && !password) {
    document.getElementById("binary-password-feedback").textContent =
      "Por favor ingrese una contraseña para el cifrado.";
    document.getElementById("binary-password").classList.add("is-invalid");
    document.getElementById("binary-password").focus();
    return;
  }

  // Configurar permisos en los metadatos
  metadata.security = {
    public: isPublic,
    encrypted: isEncrypted,
    allowedUsers: allowedUsers
      ? allowedUsers.split(",").map((u) => u.trim())
      : [],
    owner: currentUsername || "admin",
  };

  // Mostrar spinner
  const spinner = document.getElementById("save-binary-spinner");
  spinner.classList.remove("hidden");

  try {
    // Subir archivo
    const file = fileInput.files[0];
    const result = await uploadBinaryToCollection(collection, file, metadata);

    // Ocultar spinner
    spinner.classList.add("hidden");

    // Cerrar modal
    const binaryModal = bootstrap.Modal.getInstance(
      document.getElementById("binary-modal")
    );
    binaryModal.hide();

    // Mostrar mensaje de éxito
    showAlert(
      `Archivo ${file.name} subido exitosamente (ID: ${result.id || "N/A"})`,
      "success"
    );

    // Actualizar la lista de documentos
    refreshDocuments();
  } catch (error) {
    // Ocultar spinner
    spinner.classList.add("hidden");

    // Mostrar mensaje de error
    showAlert(`Error al subir archivo: ${error.message}`, "danger");
  }
}

// --- Operaciones CRUD (Colecciones/Documentos) ---

/**
 * Valida los datos del formulario y crea un nuevo documento.
 */
async function validateAndCreate() {
  // Obtener los datos del formulario del modal
  const collection = document
    .getElementById("document-collection")
    .value.trim();
  const dataStr = document.getElementById("document-data").value.trim();

  // Validación mejorada del nombre de la colección
  const collectionInput = document.getElementById("document-collection");
  const collectionFeedback = document.getElementById(
    "document-collection-feedback"
  );

  if (!collection) {
    collectionFeedback.textContent = "Por favor especifique una colección.";
    collectionInput.classList.add("is-invalid");
    collectionInput.focus();
    return;
  } else if (!/^[a-zA-Z0-9_-]+$/.test(collection)) {
    collectionFeedback.textContent =
      "El nombre de la colección solo puede contener letras, números, guiones y guiones bajos.";
    collectionInput.classList.add("is-invalid");
    collectionInput.focus();
    return;
  } else {
    collectionInput.classList.remove("is-invalid");
    collectionInput.classList.add("is-valid");
  }

  // Validación mejorada de los datos JSON
  const dataInput = document.getElementById("document-data");
  const dataFeedback = document.getElementById("document-data-feedback");

  if (!dataStr.trim()) {
    dataFeedback.textContent = "Por favor ingrese datos en formato JSON.";
    dataInput.classList.add("is-invalid");
    dataInput.focus();
    return;
  }

  let data;
  try {
    data = JSON.parse(dataStr);

    // Validar que sea un objeto y no un valor primitivo o un array
    if (typeof data !== "object" || data === null || Array.isArray(data)) {
      dataFeedback.textContent = "Los datos deben ser un objeto JSON válido.";
      dataInput.classList.add("is-invalid");
      dataInput.focus();
      return;
    }

    // Validar que no esté vacío
    if (Object.keys(data).length === 0) {
      dataFeedback.textContent = "El objeto JSON no puede estar vacío.";
      dataInput.classList.add("is-invalid");
      dataInput.focus();
      return;
    }
  } catch (error) {
    dataFeedback.textContent = `JSON inválido: ${error.message}`;
    dataInput.classList.add("is-invalid");
    dataInput.focus();
    return;
  }

  // Limpiar validación
  dataInput.classList.remove("is-invalid");
  dataInput.classList.add("is-valid");

  // Mostrar spinner
  const spinner = document.getElementById("save-document-spinner");
  spinner.classList.remove("hidden");

  try {
    // Invalidar caché relacionada con esta colección antes de crear un nuevo documento
    cache.invalidatePattern(
      new RegExp(`${API_URL}\/collections\/${collection}`)
    );

    const result = await makeApiRequest(
      `${API_URL}/collections/${collection}`,
      {
        method: "POST",
        body: JSON.stringify(data),
        // Deshabilitar caché para operaciones de escritura
        useCache: false,
      }
    );

    // Ocultar spinner
    spinner.classList.add("hidden");

    // Cerrar modal
    const documentModal = bootstrap.Modal.getInstance(
      document.getElementById("document-modal")
    );
    documentModal.hide();

    showAlert(
      `Documento creado exitosamente (ID: ${result.id || "N/A"})`,
      "success"
    );

    // Actualizar la colección actual si es necesario
    if (collection !== collectionInput.value) {
      collectionInput.value = collection;
    }

    refreshDocuments(); // Actualiza la lista de documentos
  } catch (error) {
    // Ocultar spinner
    spinner.classList.add("hidden");
    showAlert(`Error al crear documento: ${error.message}`, "danger");
  }
}

/**
 * Obtiene un documento para edición.
 * @param {string} id - ID del documento.
 * @param {string} collection - Nombre de la colección.
 */
async function getDocumentForEdit(id, collection) {
  try {
    // Obtener el documento de la API
    const docData = await makeApiRequest(
      `${API_URL}/collections/${collection}/${id}`
    );

    if (!docData) {
      throw new Error("No se pudo obtener el documento");
    }

    console.log("Documento obtenido para edición:", docData);

    // Abrir el modal de edición
    const documentModal = new bootstrap.Modal(
      document.getElementById("document-modal")
    );

    // Configurar el modal para edición
    document.getElementById("document-modal-label").textContent =
      "Editar documento";
    document.getElementById("document-id").value = id;
    document.getElementById("document-collection").value = collection;
    document.getElementById("document-collection").readOnly = true; // No permitir cambiar la colección en edición
    document.getElementById("document-data").value = JSON.stringify(
      docData.data,
      null,
      2
    );

    // Cambiar el comportamiento del botón guardar
    const saveButton = document.getElementById("save-document-btn");
    saveButton.onclick = () => updateDocument(id, collection);

    // Mostrar el modal
    documentModal.show();
  } catch (error) {
    showAlert(
      `Error al obtener documento para edición: ${error.message}`,
      "danger"
    );
  }
}

/**
 * Actualiza un documento existente.
 * @param {string} id - ID del documento.
 * @param {string} collection - Nombre de la colección.
 */
async function updateDocument(id, collection) {
  try {
    // Obtener los datos del formulario
    const dataStr = document.getElementById("document-data").value;

    // Validar JSON
    let data;
    try {
      data = JSON.parse(dataStr);
    } catch (e) {
      document.getElementById(
        "document-data-feedback"
      ).textContent = `JSON inválido: ${e.message}`;
      document.getElementById("document-data").classList.add("is-invalid");
      return;
    }

    // Limpiar validación
    document.getElementById("document-data").classList.remove("is-invalid");

    // Mostrar spinner
    const spinner = document.getElementById("save-document-spinner");
    spinner.classList.remove("hidden");

    // Invalidar caché relacionada con esta colección y documento antes de actualizar
    cache.invalidatePattern(
      new RegExp(`${API_URL}\/collections\/${collection}`)
    );
    cache.delete(`${API_URL}/collections/${collection}/${id}`);

    // Enviar petición de actualización
    await makeApiRequest(`${API_URL}/collections/${collection}/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
      useCache: false, // Deshabilitar caché para operaciones de escritura
    });

    // Ocultar spinner
    spinner.classList.add("hidden");

    // Cerrar modal
    const documentModal = bootstrap.Modal.getInstance(
      document.getElementById("document-modal")
    );
    documentModal.hide();

    // Mostrar mensaje de éxito
    showAlert(`Documento actualizado exitosamente`, "success");

    // Refrescar lista de documentos
    refreshDocuments();
  } catch (error) {
    // Ocultar spinner
    document.getElementById("save-document-spinner").classList.add("hidden");

    // Mostrar error
    showAlert(`Verifica Permisos: ${error.message}`, "danger");
  }
}

/**
 * Envía la petición para eliminar un documento.
 * @param {string} id - ID del documento.
 * @param {string} collection - Nombre de la colección.
 */
async function deleteDocument(id, collection) {
  if (!collection || !id) {
    showAlert("Error interno: Falta colección o ID para eliminar.", "danger");
    return;
  }
  try {
    // Mostrar indicador de carga
    showLoading();

    // Invalidar caché relacionada con esta colección y documento antes de eliminar
    cache.invalidatePattern(
      new RegExp(`${API_URL}\/collections\/${collection}`)
    );
    cache.delete(`${API_URL}/collections/${collection}/${id}`);

    // Enviar petición de eliminación
    await makeApiRequest(`${API_URL}/collections/${collection}/${id}`, {
      method: "DELETE",
      useCache: false, // Deshabilitar caché para operaciones de escritura
    });

    // Mostrar mensaje de éxito
    showAlert(`Documento con ID ${id} eliminado exitosamente.`, "success");

    // Actualizar la lista de documentos
    refreshDocuments();
  } catch (error) {
    showAlert(`Error al eliminar documento: ${error.message}`, "danger");
  } finally {
    hideLoading();
  }
}

/**
 * Obtiene y muestra los documentos de la colección seleccionada.
 */
async function refreshDocuments() {
  const collection = collectionInput.value.trim();
  if (!documentsList) return; // Salir si el elemento no existe

  if (!collection) {
    documentsList.innerHTML =
      '<p class="text-muted">Seleccione o escriba una colección para ver sus documentos.</p>';
    return;
  }

  // Actualizar el badge de la colección
  const collectionBadge = document.getElementById("collection-badge");
  if (collectionBadge) {
    collectionBadge.textContent = collection;
  }

  documentsList.innerHTML = `
    <div class="d-flex justify-content-center">
      <div class="spinner-border text-primary" role="status">
        <span class="visually-hidden">Cargando...</span>
      </div>
    </div>
  `; // Estado de carga

  try {
    console.log(`Obteniendo documentos de la colección: ${collection}`);
    console.log(`URL: ${API_URL}/collections/${collection}`);

    const documents = await makeApiRequest(
      `${API_URL}/collections/${collection}`
    );

    console.log(`Documentos obtenidos: ${JSON.stringify(documents)}`);

    documentsList.innerHTML = ""; // Limpia antes de añadir nuevos

    if (!Array.isArray(documents) || documents.length === 0) {
      documentsList.innerHTML = `
        <div class="alert alert-info">
          <i class="bi bi-info-circle"></i> No hay documentos en la colección "${collection}".
          <button id="create-first-doc-btn" class="btn btn-sm btn-primary ms-3">
            <i class="bi bi-plus-circle"></i> Crear primer documento
          </button>
        </div>
      `;

      // Agregar evento al botón de crear primer documento
      const createFirstDocBtn = document.getElementById("create-first-doc-btn");
      if (createFirstDocBtn) {
        createFirstDocBtn.addEventListener("click", () => {
          // Abrir el modal de creación
          // Obtener el modal
          const modalElement = document.getElementById("document-modal");
          const documentModal = new bootstrap.Modal(modalElement);

          // Configurar el modal para creación
          document.getElementById("document-modal-label").textContent =
            "Nuevo documento";
          document.getElementById("document-id").value = "";
          document.getElementById("document-collection").value = collection;
          document.getElementById("document-collection").readOnly = false;

          // Plantilla para usuarios si es la colección "usuarios"
          if (collection === "usuarios") {
            document.getElementById("document-data").value = JSON.stringify(
              {
                nombre: "Nuevo Usuario",
                email: "usuario@ejemplo.com",
                edad: 30,
                activo: true,
                roles: ["usuario"],
              },
              null,
              2
            );
          } else {
            document.getElementById("document-data").value = JSON.stringify(
              {
                campo1: "valor1",
                campo2: "valor2",
              },
              null,
              2
            );
          }

          // Cambiar el comportamiento del botón guardar
          const saveButton = document.getElementById("save-document-btn");
          saveButton.onclick = validateAndCreate;

          // Mostrar el modal
          documentModal.show();
        });
      }

      return;
    }

    // Mostrar el número de documentos
    const docCount = documents.length;
    if (collectionBadge) {
      collectionBadge.textContent = `${collection} (${docCount})`;
    }

    // Si es la colección usuarios, mostrar en formato de tabla
    if (collection === "usuarios") {
      // Crear tabla
      const table = document.createElement("table");
      table.className = "table table-striped table-hover";

      // Crear encabezado
      const thead = document.createElement("thead");
      thead.innerHTML = `
        <tr>
          <th>ID</th>
          <th>Nombre</th>
          <th>Email</th>
          <th>Edad</th>
          <th>Estado</th>
          <th>Acciones</th>
        </tr>
      `;
      table.appendChild(thead);

      // Crear cuerpo de la tabla
      const tbody = document.createElement("tbody");
      documents.forEach((doc) => {
        const tr = document.createElement("tr");
        tr.innerHTML = `
          <td><small class="text-muted">${doc.id}</small></td>
          <td>${doc.data.nombre || "N/A"}</td>
          <td>${doc.data.email || "N/A"}</td>
          <td>${doc.data.edad || "N/A"}</td>
          <td>${
            doc.data.activo
              ? '<span class="badge bg-success">Activo</span>'
              : '<span class="badge bg-secondary">Inactivo</span>'
          }</td>
          <td>
            <!-- Verificar si es un documento binario -->
            ${
              isBinaryDocument(doc)
                ? `
            <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-success me-1 download-binary-button">
              <i class="bi bi-download"></i>
            </button>
            <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-info me-1 view-binary-button">
              <i class="bi bi-eye"></i>
            </button>
            `
                : `
            <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-primary me-1 edit-button">
              <i class="bi bi-pencil"></i>
            </button>
            `
            }
            <button data-doc-id="${
              doc.id
            }" data-collection="${collection}" class="btn btn-sm btn-danger delete-button">
              <i class="bi bi-trash"></i>
            </button>
          </td>
        `;
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);

      documentsList.appendChild(table);
    } else {
      // Para otras colecciones, mostrar como tarjetas
      documents.forEach((doc) => {
        const docElement = document.createElement("div");
        docElement.className =
          "list-item document-item mb-3 p-3 border rounded shadow-sm"; // Clases ejemplo

        // Verificar si es un documento binario
        const isBinary =
          doc.data &&
          (doc.data.mimetype || doc.data.filename) &&
          (doc.data.binary || doc.data.isMultipart === true || doc.data.size);

        console.log(`Documento ${doc.id}: ¿Es binario? ${isBinary}`, doc.data);

        let docContent = "";
        if (isBinary) {
          // Mostrar información del archivo binario
          const fileIcon = getFileIcon(
            doc.data.mimetype || "application/octet-stream"
          );
          const fileSize = formatFileSize(doc.data.size || 0);
          docContent = `
            <div class="binary-file-info d-flex align-items-center mb-2">
              <i class="bi ${fileIcon} fs-2 me-2"></i>
              <div class="flex-grow-1">
                <div class="fw-bold">${
                  doc.data.filename || "Archivo sin nombre"
                }</div>
                <div class="text-muted small">${
                  doc.data.mimetype || "application/octet-stream"
                } - ${fileSize}</div>
              </div>
              <div class="binary-actions">
                <button data-doc-id="${
                  doc.id
                }" data-collection="${collection}" class="btn btn-sm btn-success download-binary-button">
                  <i class="bi bi-download"></i>
                </button>
                <button data-doc-id="${
                  doc.id
                }" data-collection="${collection}" class="btn btn-sm btn-info view-binary-button">
                  <i class="bi bi-eye"></i>
                </button>
              </div>
            </div>
            <div class="binary-metadata">
              <pre class="document-data-pre bg-light p-2 rounded small">${JSON.stringify(
                // Excluir el contenido binario para no sobrecargar la UI
                { ...doc.data, binary: "[CONTENIDO BINARIO]" },
                null,
                2
              )}</pre>
            </div>
          `;
        } else {
          // Mostrar documento normal
          docContent = `<pre class="document-data-pre bg-light p-2 rounded small">${JSON.stringify(
            doc.data,
            null,
            2
          )}</pre>`;
        }

        docElement.innerHTML = `
                  <div class="document-id-badge mb-2">
                      <span class="badge bg-secondary">ID: ${doc.id}</span>
                  </div>
                  ${docContent}
                  <div class="document-actions mt-2">
                      ${
                        isBinary
                          ? `
                      <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-success me-2 download-binary-button">
                        <i class="bi bi-download"></i> Descargar
                      </button>
                      <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-info me-2 view-binary-button">
                        <i class="bi bi-eye"></i> Ver
                      </button>
                      `
                          : `
                      <button data-doc-id="${doc.id}" data-collection="${collection}" class="btn btn-sm btn-primary me-2 edit-button">
                        <i class="bi bi-pencil"></i> Editar
                      </button>
                      `
                      }
                      <button data-doc-id="${
                        doc.id
                      }" data-collection="${collection}" class="btn btn-sm btn-danger delete-button">
                        <i class="bi bi-trash"></i> Eliminar
                      </button>
                  </div>
              `;
        documentsList.appendChild(docElement);
      });
    }
  } catch (error) {
    showAlert(
      `Error al obtener documentos de '${collection}': ${error.message}`,
      "danger"
    );
    documentsList.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle"></i> Error al cargar documentos: ${error.message}
      </div>
    `;
  }
}

/**
 * Carga y muestra la lista de usuarios.
 */
async function loadUsers() {
  const usersContainer = document.getElementById("users-container");
  if (!usersContainer) {
    console.error("Contenedor de usuarios no encontrado");
    return;
  }

  // Mostrar indicador de carga
  usersContainer.innerHTML = `
    <div class="d-flex justify-content-center">
      <div class="spinner-border text-primary" role="status">
        <span class="visually-hidden">Cargando usuarios...</span>
      </div>
    </div>
  `;

  try {
    // Obtener usuarios de la API
    const users = await makeApiRequest(`${API_URL}/users`);

    if (!Array.isArray(users) || users.length === 0) {
      usersContainer.innerHTML = `
        <div class="alert alert-info">
          <i class="bi bi-info-circle"></i> No hay usuarios registrados.
          <button id="create-first-user-btn" class="btn btn-sm btn-primary ms-3">
            <i class="bi bi-person-plus"></i> Crear primer usuario
          </button>
        </div>
      `;

      // Agregar evento al botón de crear primer usuario
      const createFirstUserBtn = document.getElementById(
        "create-first-user-btn"
      );
      if (createFirstUserBtn) {
        createFirstUserBtn.addEventListener("click", showUserModal);
      }

      return;
    }

    // Crear tabla de usuarios
    const table = document.createElement("table");
    table.className = "table table-striped table-hover";

    // Crear encabezado
    const thead = document.createElement("thead");
    thead.innerHTML = `
      <tr>
        <th>ID</th>
        <th>Usuario</th>
        <th>Roles</th>
        <th>Claves API</th>
        <th>Creado</th>
        <th>Acciones</th>
      </tr>
    `;
    table.appendChild(thead);

    // Crear cuerpo de la tabla
    const tbody = document.createElement("tbody");
    users.forEach((user) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td><small class="text-muted">${user.id}</small></td>
        <td>${user.username}</td>
        <td>${user.roles
          .map((role) => `<span class="badge bg-info me-1">${role}</span>`)
          .join("")}</td>
        <td>${user.api_keys ? user.api_keys.length : 0} claves</td>
        <td>${new Date(user.created_at).toLocaleString()}</td>
        <td>
          <button data-user-id="${
            user.id
          }" class="btn btn-sm btn-primary me-1 edit-user-button">
            <i class="bi bi-pencil"></i>
          </button>
          <button data-user-id="${
            user.id
          }" class="btn btn-sm btn-danger delete-user-button">
            <i class="bi bi-trash"></i>
          </button>
          <button data-user-id="${
            user.id
          }" class="btn btn-sm btn-secondary apikey-user-button">
            <i class="bi bi-key"></i>
          </button>
        </td>
      `;
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);

    // Limpiar y añadir la tabla al contenedor
    usersContainer.innerHTML = "";
    usersContainer.appendChild(table);

    // Agregar eventos a los botones
    document.querySelectorAll(".edit-user-button").forEach((button) => {
      button.addEventListener("click", () => {
        const userId = button.getAttribute("data-user-id");
        editUser(userId);
      });
    });

    document.querySelectorAll(".delete-user-button").forEach((button) => {
      button.addEventListener("click", () => {
        const userId = button.getAttribute("data-user-id");
        if (confirm(`¿Está seguro de eliminar el usuario con ID ${userId}?`)) {
          deleteUser(userId);
        }
      });
    });

    document.querySelectorAll(".apikey-user-button").forEach((button) => {
      button.addEventListener("click", () => {
        const userId = button.getAttribute("data-user-id");
        showApiKeyModal(userId);
      });
    });
  } catch (error) {
    usersContainer.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle"></i> Error al cargar usuarios: ${error.message}
      </div>
    `;
  }
}

/**
 * Muestra el modal para crear o editar un usuario.
 * @param {string} [userId] - ID del usuario a editar (si es undefined, se crea un nuevo usuario).
 */
async function showUserModal(userId) {
  // Obtener el modal
  const modalElement = document.getElementById("user-modal");
  const userModal = new bootstrap.Modal(modalElement);

  // Configurar el modal
  document.getElementById("user-modal-label").textContent = userId
    ? "Editar usuario"
    : "Nuevo usuario";
  document.getElementById("user-id").value = userId || "";

  // Si es edición, cargar datos del usuario
  if (userId) {
    try {
      const user = await makeApiRequest(`${API_URL}/users/${userId}`);
      document.getElementById("user-username").value = user.username;
      document.getElementById("user-password").value = ""; // No mostrar contraseña
      document.getElementById("user-password").placeholder =
        "Dejar en blanco para mantener la actual";

      // Marcar roles
      document.getElementById("role-admin").checked =
        user.roles.includes("admin");
      document.getElementById("role-reader").checked =
        user.roles.includes("reader");
      document.getElementById("role-writer").checked =
        user.roles.includes("writer");
    } catch (error) {
      showAlert(
        `Error al cargar datos del usuario: ${error.message}`,
        "danger"
      );
      return;
    }
  } else {
    // Limpiar formulario para nuevo usuario
    document.getElementById("user-username").value = "";
    document.getElementById("user-password").value = "";
    document.getElementById("user-password").placeholder = "Contraseña";
    document.getElementById("role-admin").checked = false;
    document.getElementById("role-reader").checked = true;
    document.getElementById("role-writer").checked = false;
  }

  // Configurar evento del botón guardar
  const saveButton = document.getElementById("save-user-btn");
  saveButton.onclick = () => saveUser(userId);

  // Mostrar el modal
  userModal.show();
}

/**
 * Guarda un usuario (crea o actualiza).
 * @param {string} [userId] - ID del usuario a actualizar (si es undefined, se crea un nuevo usuario).
 */
async function saveUser(userId) {
  // Obtener datos del formulario
  const username = document.getElementById("user-username").value.trim();
  const password = document.getElementById("user-password").value;
  const roles = [];

  if (document.getElementById("role-admin").checked) roles.push("admin");
  if (document.getElementById("role-reader").checked) roles.push("reader");
  if (document.getElementById("role-writer").checked) roles.push("writer");

  // Validar datos
  if (!username) {
    showAlert("El nombre de usuario es obligatorio", "danger");
    return;
  }

  if (!userId && !password) {
    showAlert("La contraseña es obligatoria para nuevos usuarios", "danger");
    return;
  }

  if (roles.length === 0) {
    showAlert("Debe seleccionar al menos un rol", "danger");
    return;
  }

  // Preparar datos para enviar
  const userData = {
    username,
    roles,
  };

  // Añadir contraseña solo si se proporciona
  if (password) {
    userData.password = password;
  }

  // Imprimir datos para depuración
  console.log("Datos a enviar:", userData);

  try {
    // Mostrar spinner
    const spinner = document.getElementById("save-user-spinner");
    spinner.classList.remove("hidden");

    if (userId) {
      // Actualizar usuario existente
      console.log(`Enviando PUT a ${API_URL}/users/${userId}`);
      const response = await makeApiRequest(`${API_URL}/users/${userId}`, {
        method: "PUT",
        body: JSON.stringify(userData),
      });
      console.log("Respuesta de actualización:", response);
      showAlert("Usuario actualizado exitosamente", "success");
    } else {
      // Crear nuevo usuario
      console.log(`Enviando POST a ${API_URL}/users`);
      const response = await makeApiRequest(`${API_URL}/users`, {
        method: "POST",
        body: JSON.stringify(userData),
      });
      console.log("Respuesta de creación:", response);
      showAlert("Usuario creado exitosamente", "success");
    }

    // Ocultar spinner
    spinner.classList.add("hidden");

    // Cerrar modal
    const userModal = bootstrap.Modal.getInstance(
      document.getElementById("user-modal")
    );
    userModal.hide();

    // Recargar lista de usuarios
    loadUsers();
  } catch (error) {
    // Ocultar spinner
    document.getElementById("save-user-spinner").classList.add("hidden");
    showAlert(`Error al guardar usuario: ${error.message}`, "danger");
  }
}

/**
 * Elimina un usuario.
 * @param {string} userId - ID del usuario a eliminar.
 */
async function deleteUser(userId) {
  try {
    // Mostrar indicador de carga
    showLoading();

    // Enviar petición de eliminación
    await makeApiRequest(`${API_URL}/users/${userId}`, {
      method: "DELETE",
    });

    // Mostrar mensaje de éxito
    showAlert("Usuario eliminado exitosamente", "success");

    // Recargar lista de usuarios
    loadUsers();
  } catch (error) {
    showAlert(`Error al eliminar usuario: ${error.message}`, "danger");
  } finally {
    hideLoading();
  }
}

/**
 * Muestra el modal para crear una clave API.
 * @param {string} userId - ID del usuario.
 */
async function showApiKeyModal(userId) {
  // Obtener el modal
  const modalElement = document.getElementById("apikey-modal");
  const apikeyModal = new bootstrap.Modal(modalElement);

  // Configurar el modal
  document.getElementById("apikey-user-id").value = userId;
  document.getElementById("apikey-name").value = "";
  document.getElementById("apikey-days").value = "30";

  // Configurar evento del botón crear
  const createButton = document.getElementById("save-apikey-btn");
  createButton.onclick = createApiKey;

  // Mostrar el modal
  apikeyModal.show();
}

/**
 * Crea una nueva clave API.
 */
async function createApiKey() {
  const userId = document.getElementById("apikey-user-id").value;
  const name = document.getElementById("apikey-name").value.trim();
  const validDays = parseInt(
    document.getElementById("apikey-valid-days").value
  );

  // Validar datos
  if (!name) {
    showAlert("El nombre de la clave API es obligatorio", "danger");
    return;
  }

  if (isNaN(validDays) || validDays <= 0) {
    showAlert("Los días de validez deben ser un número positivo", "danger");
    return;
  }

  try {
    // Mostrar spinner
    const spinner = document.getElementById("save-apikey-spinner");
    spinner.classList.remove("hidden");

    // Crear clave API
    const result = await makeApiRequest(`${API_URL}/users/${userId}/apikeys`, {
      method: "POST",
      body: JSON.stringify({
        name,
        valid_days: validDays,
      }),
    });

    // Ocultar spinner
    spinner.classList.add("hidden");

    // Cerrar modal
    const apikeyModal = bootstrap.Modal.getInstance(
      document.getElementById("apikey-modal")
    );
    apikeyModal.hide();

    // Mostrar la clave generada
    showAlert(
      `Clave API creada exitosamente: <br><code>${result.token}</code><br>Guárdela en un lugar seguro, no se mostrará nuevamente.`,
      "success"
    );

    // Recargar lista de usuarios
    loadUsers();
  } catch (error) {
    // Ocultar spinner
    document.getElementById("save-apikey-spinner").classList.add("hidden");
    showAlert(`Error al crear clave API: ${error.message}`, "danger");
  }
}

/**
 * Carga y muestra la lista de copias de seguridad disponibles.
 */
async function listBackups() {
  const backupsContainer = document.getElementById("backups-container");
  if (!backupsContainer) {
    console.error("Contenedor de backups no encontrado");
    return;
  }

  // Mostrar indicador de carga
  backupsContainer.innerHTML = `
    <div class="d-flex justify-content-center">
      <div class="spinner-border text-primary" role="status">
        <span class="visually-hidden">Cargando copias de seguridad...</span>
      </div>
    </div>
  `;

  try {
    // Obtener backups de la API
    const backups = await makeApiRequest(`${API_URL}/backups`);

    if (!Array.isArray(backups) || backups.length === 0) {
      backupsContainer.innerHTML = `
        <div class="alert alert-info">
          <i class="bi bi-info-circle"></i> No hay copias de seguridad disponibles.
        </div>
      `;
      return;
    }

    // Crear lista de backups
    const backupsList = document.createElement("div");
    backupsList.className = "list-group";

    backups.forEach((backup) => {
      // Extraer fecha del nombre (formato: backup_YYYYMMDD_HHMMSS)
      let displayDate = backup;
      if (backup.startsWith("backup_")) {
        const datePart = backup.substring(7); // Quitar "backup_"
        try {
          // Intentar formatear la fecha
          const year = datePart.substring(0, 4);
          const month = datePart.substring(4, 6);
          const day = datePart.substring(6, 8);
          const hour = datePart.substring(9, 11);
          const minute = datePart.substring(11, 13);
          const second = datePart.substring(13, 15);
          displayDate = `${day}/${month}/${year} ${hour}:${minute}:${second}`;
        } catch (e) {
          // Si hay error en el formato, usar el nombre original
          console.warn("Error al formatear fecha de backup:", e);
        }
      }

      const backupItem = document.createElement("div");
      backupItem.className =
        "list-group-item list-group-item-action d-flex justify-content-between align-items-center";
      backupItem.innerHTML = `
        <div>
          <h5 class="mb-1">${displayDate}</h5>
          <small class="text-muted">${backup}</small>
        </div>
        <div>
          <button data-backup-name="${backup}" class="btn btn-sm btn-success me-2 restore-backup-button">
            <i class="bi bi-cloud-download"></i> Restaurar
          </button>
          <button data-backup-name="${backup}" class="btn btn-sm btn-danger delete-backup-button">
            <i class="bi bi-trash"></i> Eliminar
          </button>
        </div>
      `;
      backupsList.appendChild(backupItem);
    });

    // Limpiar y añadir la lista al contenedor
    backupsContainer.innerHTML = "";
    backupsContainer.appendChild(backupsList);

    // Agregar eventos a los botones
    document.querySelectorAll(".restore-backup-button").forEach((button) => {
      button.addEventListener("click", () => {
        const backupName = button.getAttribute("data-backup-name");
        if (
          confirm(
            `¿Está seguro de restaurar la copia de seguridad ${backupName}? Esta acción reemplazará todos los datos actuales.`
          )
        ) {
          restoreBackup(backupName);
        }
      });
    });

    document.querySelectorAll(".delete-backup-button").forEach((button) => {
      button.addEventListener("click", () => {
        const backupName = button.getAttribute("data-backup-name");
        if (
          confirm(
            `¿Está seguro de eliminar la copia de seguridad ${backupName}? Esta acción no se puede deshacer.`
          )
        ) {
          deleteBackup(backupName);
        }
      });
    });
  } catch (error) {
    backupsContainer.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle"></i> Error al cargar copias de seguridad: ${error.message}
      </div>
    `;
  }
}

/**
 * Crea una nueva copia de seguridad.
 */
async function createBackup() {
  try {
    // Mostrar indicador de carga
    showLoading();
    showAlert("Creando copia de seguridad...", "info");

    // Crear backup
    const result = await makeApiRequest(`${API_URL}/backups`, {
      method: "POST",
    });

    // Mostrar mensaje de éxito
    showAlert(
      `Copia de seguridad creada exitosamente: ${result.backup_name}`,
      "success"
    );

    // Recargar lista de backups
    listBackups();
  } catch (error) {
    showAlert(`Error al crear copia de seguridad: ${error.message}`, "danger");
  } finally {
    hideLoading();
  }
}

/**
 * Restaura una copia de seguridad.
 * @param {string} backupName - Nombre de la copia de seguridad a restaurar.
 */
async function restoreBackup(backupName) {
  try {
    // Mostrar indicador de carga
    showLoading();
    showAlert(`Restaurando copia de seguridad ${backupName}...`, "info");

    // Restaurar backup
    await makeApiRequest(`${API_URL}/backups/${backupName}`, {
      method: "POST",
    });

    // Mostrar mensaje de éxito
    showAlert("Copia de seguridad restaurada exitosamente", "success");

    // Recargar lista de backups y documentos
    listBackups();
    refreshDocuments();
  } catch (error) {
    showAlert(
      `Error al restaurar copia de seguridad: ${error.message}`,
      "danger"
    );
  } finally {
    hideLoading();
  }
}

/**
 * Elimina una copia de seguridad.
 * @param {string} backupName - Nombre de la copia de seguridad a eliminar.
 */
async function deleteBackup(backupName) {
  try {
    // Mostrar indicador de carga
    showLoading();
    showAlert(`Eliminando copia de seguridad ${backupName}...`, "info");

    // Eliminar backup
    await makeApiRequest(`${API_URL}/backups/${backupName}`, {
      method: "DELETE",
    });

    // Mostrar mensaje de éxito
    showAlert("Copia de seguridad eliminada exitosamente", "success");

    // Recargar lista de backups
    listBackups();
  } catch (error) {
    showAlert(
      `Error al eliminar copia de seguridad: ${error.message}`,
      "danger"
    );
  } finally {
    hideLoading();
  }
}

// --- WebSocket ---

/**
 * Establece la conexión WebSocket.
 */
function connectWebSocket() {
  if (!token) {
    console.warn("Intento de conexión WebSocket sin token.");
    return;
  }
  // Evita múltiples conexiones si ya existe una o se está conectando
  if (
    ws &&
    (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)
  ) {
    console.log("Conexión WebSocket ya existente o en proceso.");
    return;
  }

  // Cierra conexión anterior si estaba cerrada incorrectamente
  if (ws && ws.readyState === WebSocket.CLOSED) {
    ws = null;
  }

  console.log("Intentando conectar WebSocket...");
  ws = new WebSocket(`${WS_URL}?token=${token}`);

  ws.onopen = () => {
    console.log("Conexión WebSocket establecida.");
    showAlert("Conexión WebSocket establecida", "success");
    // Intenta suscribirse automáticamente si hay una colección en el input de WS
    const wsCollection = wsCollectionInput
      ? wsCollectionInput.value.trim()
      : null;
    if (wsCollection) {
      subscribeToCollection(wsCollection);
    } else {
      if (currentWsSubscriptionSpan)
        currentWsSubscriptionSpan.textContent = "Conectado, no suscrito";
    }
  };

  ws.onmessage = (event) => {
    try {
      const update = JSON.parse(event.data);
      console.log("Mensaje WebSocket recibido:", update);
      if (!realTimeUpdatesList) return;

      const updateElement = document.createElement("div");
      // Clases para estilizar las actualizaciones
      updateElement.className =
        "list-item ws-update-item mb-2 p-2 border rounded small";
      // Muestra más detalles del mensaje
      updateElement.innerHTML = `
                <div>
                    <span class="badge bg-info me-1">${
                      update.action || "UPDATE"
                    }</span>
                    en <strong>${update.collection || "N/A"}</strong>
                    ${
                      update.docId
                        ? `<span class="text-muted fst-italic"> (ID: ${update.docId})</span>`
                        : ""
                    }
                    <span class="text-muted float-end">${new Date().toLocaleTimeString()}</span>
                </div>
                <pre class="ws-data-pre bg-light p-1 rounded mt-1">${JSON.stringify(
                  update.data || update,
                  null,
                  2
                )}</pre>
            `;
      // Inserta al principio de la lista
      realTimeUpdatesList.insertBefore(
        updateElement,
        realTimeUpdatesList.firstChild
      );

      // Limita el número de mensajes mostrados para evitar sobrecargar el DOM
      const maxMessages = 50;
      while (realTimeUpdatesList.children.length > maxMessages) {
        realTimeUpdatesList.removeChild(realTimeUpdatesList.lastChild);
      }

      // Si la actualización afecta a la colección que se está viendo, refresca la lista de documentos
      const currentViewedCollection = collectionInput
        ? collectionInput.value.trim()
        : null;
      if (update.collection && update.collection === currentViewedCollection) {
        console.log(
          `Actualización recibida para la colección actual '${currentViewedCollection}'. Refrescando lista...`
        );
        refreshDocuments();
      }
    } catch (e) {
      console.error("Error al parsear mensaje WebSocket:", e);
      console.error("Dato recibido:", event.data);
      // Muestra el mensaje crudo si falla el parseo
      if (!realTimeUpdatesList) return;
      const errorElement = document.createElement("div");
      errorElement.className =
        "list-item ws-update-item text-danger border rounded p-2 mb-2 small";
      errorElement.textContent = `[${new Date().toLocaleTimeString()}] Error procesando mensaje: ${
        event.data
      }`;
      realTimeUpdatesList.insertBefore(
        errorElement,
        realTimeUpdatesList.firstChild
      );
    }
  };

  ws.onerror = (error) => {
    console.error("Error en WebSocket:", error);
    showAlert("Error en la conexión WebSocket. Ver consola para detalles.");
    // El evento 'onclose' se disparará después de 'onerror'
  };

  ws.onclose = (event) => {
    console.log(
      `Conexión WebSocket cerrada. Código: ${event.code}, Razón: ${
        event.reason || "Sin razón especificada"
      }`
    );
    ws = null; // Limpia la instancia

    // Evita reintentos si el cierre fue normal (logout) o por error de autenticación/política
    const intentionalCloseCodes = [1000, 1008]; // 1000 = Normal, 1008 = Policy Violation
    const shouldRetry = !intentionalCloseCodes.includes(event.code) && token; // Solo reintenta si no fue intencional y aún hay token

    if (shouldRetry) {
      showAlert(
        `Conexión WebSocket perdida (Código: ${event.code}). Reintentando en 5 segundos...`,
        "warning"
      );
      console.log("Reintentando conexión WebSocket en 5 segundos...");
      setTimeout(connectWebSocket, 5000); // Reintento simple
      // Para producción, considera usar backoff exponencial
    } else {
      showAlert(`Conexión WebSocket cerrada (Código: ${event.code})`, "info");
      if (currentWsSubscriptionSpan)
        currentWsSubscriptionSpan.textContent = "Desconectado";
    }
  };
}

/**
 * Envía un mensaje para suscribirse a actualizaciones de una colección.
 * @param {string} collection - La colección a la que suscribirse.
 */
function subscribeToCollection(collection) {
  if (!collection) {
    showAlert("Por favor especifique una colección para suscribirse.");
    return;
  }

  if (!ws || ws.readyState !== WebSocket.OPEN) {
    showAlert("No hay conexión WebSocket. Intente reconectar.", "warning");
    return;
  }

  // Envía mensaje de suscripción
  const subscriptionMsg = {
    action: "subscribe",
    collection: collection,
  };

  try {
    ws.send(JSON.stringify(subscriptionMsg));
    showAlert(`Suscrito a actualizaciones de '${collection}'`, "success");

    // Actualiza UI para mostrar suscripción activa
    if (currentWsSubscriptionSpan) {
      currentWsSubscriptionSpan.textContent = `Suscrito a: ${collection}`;
      currentWsSubscriptionSpan.className = "badge bg-success badge-live";
    }

    // Añade badge de suscripción si no existe
    if (subscriptionsContainer) {
      // Verifica si ya existe un badge para esta colección
      const existingBadge = document.querySelector(
        `.subscription-badge[data-collection="${collection}"]`
      );
      if (!existingBadge) {
        const badge = document.createElement("span");
        badge.className =
          "badge bg-info me-2 mb-2 subscription-badge badge-live";
        badge.setAttribute("data-collection", collection);
        badge.innerHTML = `
                    ${collection}
                    <button type="button" class="btn-close btn-close-white ms-1"
                     style="font-size: 0.5rem;" aria-label="Cancelar suscripción"></button>
                `;
        // Añade evento para cancelar suscripción
        badge.querySelector(".btn-close").addEventListener("click", () => {
          // Cancelar suscripción
          const unsubMsg = {
            action: "unsubscribe",
            collection: collection,
          };

          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(unsubMsg));
            showAlert(`Suscripción a '${collection}' cancelada`, "info");
          }

          badge.remove();
        });
        subscriptionsContainer.appendChild(badge);
      }
    }
  } catch (error) {
    showAlert(`Error al suscribirse: ${error.message}`, "danger");
  }
}

// Inicialización cuando el DOM está listo
document.addEventListener("DOMContentLoaded", () => {
  // Inicializar referencias a elementos DOM
  loginSection = document.getElementById("login-panel");
  dashboard = document.getElementById("main-panel");
  userInfo = document.querySelector(".col-md-4.text-end");
  currentUserSpan = document.getElementById("connection-status");
  usernameInput = document.getElementById("username");
  passwordInput = document.getElementById("password");
  collectionInput = document.getElementById("collection-name");
  documentDataInput = document.getElementById("document-data");
  documentsList = document.getElementById("documents-container");
  wsCollectionInput = document.getElementById("subscribe-collection");
  realTimeUpdatesList = document.getElementById("events-container");
  alertContainer = document.getElementById("toast-container");
  tabsContainer = document.getElementById("mainTab");
  tabButtons = document.querySelectorAll(".nav-link");
  backupsList = document.getElementById("backups-container");
  currentWsSubscriptionSpan = document.getElementById("websocket-status");
  connectionStatus = document.getElementById("connection-status");
  logoutBtn = document.getElementById("logout-btn");
  loginSpinner = document.getElementById("login-spinner");
  mainPanel = document.getElementById("main-panel");
  loadingIndicator = document.createElement("div"); // Crear un indicador de carga genérico
  loadingIndicator.className = "spinner-border text-primary";
  loadingIndicator.style.display = "none";
  document.body.appendChild(loadingIndicator);

  // Cargar la colección "usuarios" por defecto
  if (collectionInput) {
    collectionInput.value = "usuarios";
  }

  // Asigna la función handleLogin al evento 'submit' del formulario de login
  const loginForm = document.getElementById("login-form");
  if (loginForm) {
    loginForm.addEventListener("submit", handleLogin);
  }

  // Asignar evento al botón de cargar colección
  const loadCollectionBtn = document.getElementById("load-collection-btn");
  if (loadCollectionBtn) {
    loadCollectionBtn.addEventListener("click", refreshDocuments);
  }

  // Asignar evento al botón de logout principal
  if (logoutBtn) {
    logoutBtn.addEventListener("click", logout);
  }

  // Asignar evento al botón de logout en la barra de navegación
  const navLogoutBtn = document.getElementById("nav-logout-btn");
  if (navLogoutBtn) {
    navLogoutBtn.addEventListener("click", logout);
  }

  // Asignar eventos a los botones de las pestañas
  tabButtons.forEach((button) => {
    button.addEventListener("click", (e) => {
      e.preventDefault();
      const tabTarget = button.getAttribute("data-bs-target").replace("#", "");
      showTab(tabTarget);
    });
  });

  // Asignar evento al botón de crear documento
  const createDocBtn = document.getElementById("create-document-btn");
  if (createDocBtn) {
    createDocBtn.addEventListener("click", () => {
      // Abrir el modal de creación
      // Obtener el modal
      const modalElement = document.getElementById("document-modal");
      const documentModal = new bootstrap.Modal(modalElement);

      // Configurar el modal para creación
      document.getElementById("document-modal-label").textContent =
        "Nuevo documento";
      document.getElementById("document-id").value = "";
      document.getElementById("document-collection").value =
        collectionInput.value;
      document.getElementById("document-collection").readOnly = false;
      document.getElementById("document-data").value = JSON.stringify(
        {
          nombre: "",
          email: "",
          edad: 0,
        },
        null,
        2
      );

      // Cambiar el comportamiento del botón guardar
      const saveButton = document.getElementById("save-document-btn");
      saveButton.onclick = validateAndCreate;

      // Mostrar el modal
      documentModal.show();
    });
  }

  // Asignar evento al botón de subir archivo binario
  const uploadBinaryBtn = document.getElementById("upload-binary-btn");
  if (uploadBinaryBtn) {
    uploadBinaryBtn.addEventListener("click", showBinaryModal);
  }

  // Asignar evento al botón de guardar archivo binario
  const saveBinaryBtn = document.getElementById("save-binary-btn");
  if (saveBinaryBtn) {
    saveBinaryBtn.addEventListener("click", validateAndUploadBinary);
  }

  // Asignar evento al botón de crear usuario
  const createUserBtn = document.getElementById("create-user-btn");
  if (createUserBtn) {
    createUserBtn.addEventListener("click", () => showUserModal());
  }

  // Asignar evento al botón de acceso directo a archivo
  const directAccessBtn = document.getElementById("direct-access-btn");
  if (directAccessBtn) {
    directAccessBtn.addEventListener("click", () => {
      // Limpiar el campo de ID
      const directFileIdInput = document.getElementById("direct-file-id");
      if (directFileIdInput) {
        directFileIdInput.value = "";
        directFileIdInput.classList.remove("is-invalid");
      }

      // Mostrar el modal
      const modalElement = document.getElementById("direct-access-modal");
      const directAccessModal = new bootstrap.Modal(modalElement);
      directAccessModal.show();
    });
  }

  // Asignar eventos a los botones del modal de acceso directo
  const downloadDirectBtn = document.getElementById("download-direct-btn");
  if (downloadDirectBtn) {
    downloadDirectBtn.addEventListener("click", () => {
      const fileId = document.getElementById("direct-file-id").value.trim();
      if (!fileId) {
        document.getElementById("direct-file-id-feedback").textContent =
          "Por favor ingrese un ID válido";
        document.getElementById("direct-file-id").classList.add("is-invalid");
        return;
      }

      // Cerrar el modal
      const directAccessModal = bootstrap.Modal.getInstance(
        document.getElementById("direct-access-modal")
      );
      directAccessModal.hide();

      // Acceder al archivo (descargar)
      accessBinaryById(fileId, false);
    });
  }

  const viewDirectBtn = document.getElementById("view-direct-btn");
  if (viewDirectBtn) {
    viewDirectBtn.addEventListener("click", () => {
      const fileId = document.getElementById("direct-file-id").value.trim();
      if (!fileId) {
        document.getElementById("direct-file-id-feedback").textContent =
          "Por favor ingrese un ID válido";
        document.getElementById("direct-file-id").classList.add("is-invalid");
        return;
      }

      // Cerrar el modal
      const directAccessModal = bootstrap.Modal.getInstance(
        document.getElementById("direct-access-modal")
      );
      directAccessModal.hide();

      // Acceder al archivo (visualizar)
      accessBinaryById(fileId, true);
    });
  }

  // Asignar evento al botón de crear copia de seguridad
  const createBackupBtn = document.getElementById("create-backup-btn");
  if (createBackupBtn) {
    createBackupBtn.addEventListener("click", createBackup);
  }

  // Asignar evento al botón de suscribirse a colección
  const subscribeBtn = document.getElementById("subscribe-btn");
  if (subscribeBtn) {
    subscribeBtn.addEventListener("click", () => {
      const collection = wsCollectionInput.value.trim();
      subscribeToCollection(collection);
    });
  }

  // Delegación de eventos para botones de editar/eliminar documentos
  if (documentsList) {
    documentsList.addEventListener("click", (e) => {
      console.log("Clic en la lista de documentos:", e.target);

      // Obtener el botón (puede ser el propio elemento o su padre si se hizo clic en el icono)
      let target = e.target;

      // Si se hizo clic en el icono, obtener el botón padre
      if (target.tagName === "I") {
        target = target.parentElement;
        console.log("Se hizo clic en un icono, usando el botón padre:", target);
      }

      if (target.classList.contains("edit-button")) {
        const docId = target.getAttribute("data-doc-id");
        const collection = target.getAttribute("data-collection");
        console.log(
          `Editando documento ${docId} de la colección ${collection}`
        );
        // Obtener el documento para edición
        getDocumentForEdit(docId, collection);
      } else if (target.classList.contains("delete-button")) {
        const docId = target.getAttribute("data-doc-id");
        const collection = target.getAttribute("data-collection");
        console.log(
          `Eliminando documento ${docId} de la colección ${collection}`
        );
        if (confirm(`¿Está seguro de eliminar el documento ${docId}?`)) {
          deleteDocument(docId, collection);
        }
      } else if (target.classList.contains("download-binary-button")) {
        console.log("Botón de descarga detectado:", target);
        const docId = target.getAttribute("data-doc-id");
        const collection = target.getAttribute("data-collection");
        console.log(
          `Descargando archivo binario ${docId} de la colección ${collection}`
        );
        openBinaryFile(collection, docId, false); // false = descargar, no visualizar
      } else if (target.classList.contains("view-binary-button")) {
        console.log("Botón de visualización detectado:", target);
        const docId = target.getAttribute("data-doc-id");
        const collection = target.getAttribute("data-collection");
        console.log(
          `Visualizando archivo binario ${docId} de la colección ${collection}`
        );
        openBinaryFile(collection, docId, true); // true = visualizar, no descargar
      } else {
        console.log("Clic en un elemento no manejado:", target);
      }
    });
  }

  // Intentar restaurar sesión desde localStorage
  const savedToken = localStorage.getItem("token");
  const savedUsername = localStorage.getItem("username");
  if (savedToken && savedUsername) {
    // Actualizar las URLs desde el formulario
    const serverUrlInput = document.getElementById("server-url");
    const wsUrlInput = document.getElementById("ws-url");

    if (serverUrlInput && serverUrlInput.value) {
      API_URL = serverUrlInput.value.trim();
      if (!API_URL.endsWith("/api")) {
        API_URL = API_URL + "/api";
      }
    }

    if (wsUrlInput && wsUrlInput.value) {
      WS_URL = wsUrlInput.value.trim();
      if (!WS_URL.endsWith("/ws")) {
        WS_URL = WS_URL + "/ws";
      }
    }

    console.log("Restaurando sesión con API URL:", API_URL);
    console.log("Restaurando sesión con WS URL:", WS_URL);

    token = savedToken;
    currentUsername = savedUsername;
    loginSection.style.display = "none";
    dashboard.style.display = "block";
    userInfo.style.display = "flex";
    currentUserSpan.textContent = `Usuario: ${currentUsername}`;

    // Mostrar el botón de cierre de sesión
    if (logoutBtn) {
      logoutBtn.classList.remove("hidden");
    }
    connectWebSocket();

    // Cargar la colección "usuarios" automáticamente
    if (collectionInput) {
      collectionInput.value = "usuarios";
      refreshDocuments();
    }
  }

  // --- Utilidades para archivos binarios ---
});
