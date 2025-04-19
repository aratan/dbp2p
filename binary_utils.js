/**
 * Utilidades para manejo de archivos binarios en DBP2P
 */

/**
 * Convierte un archivo a Base64
 * @param {File|Blob} file - El archivo o blob a convertir
 * @returns {Promise<string>} - Promesa que resuelve con el string en Base64
 */
function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.readAsDataURL(file);
    reader.onload = () => {
      // Eliminar el prefijo "data:*/*;base64," para obtener solo el contenido Base64
      const base64String = reader.result.split(",")[1];
      resolve(base64String);
    };
    reader.onerror = (error) => reject(error);
  });
}

/**
 * Calcula el hash SHA-256 de un archivo o blob
 * @param {File|Blob} file - El archivo o blob para calcular el hash
 * @returns {Promise<string>} - Promesa que resuelve con el hash en formato hexadecimal
 */
async function calculateSHA256(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.readAsArrayBuffer(file);
    reader.onload = async () => {
      try {
        // Usar la API Web Crypto para calcular el hash
        const hashBuffer = await crypto.subtle.digest("SHA-256", reader.result);

        // Convertir el ArrayBuffer a string hexadecimal
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        const hashHex = hashArray
          .map((b) => b.toString(16).padStart(2, "0"))
          .join("");

        resolve(hashHex);
      } catch (error) {
        reject(error);
      }
    };
    reader.onerror = (error) => reject(error);
  });
}

/**
 * Sube un archivo binario a una colección
 * @param {string} collection - Nombre de la colección
 * @param {File} file - Archivo a subir
 * @param {Object} metadata - Metadatos adicionales para el archivo
 * @param {Function} [progressCallback] - Función de callback para reportar progreso (0-100)
 * @returns {Promise<Object>} - Promesa que resuelve con los metadatos del archivo creado
 */
async function uploadBinaryToCollection(
  collection,
  file,
  metadata = {},
  progressCallback = null
) {
  try {
    // Tamaño máximo de fragmento (5MB)
    const CHUNK_SIZE = 5 * 1024 * 1024;

    // Verificar si el archivo es grande (>10MB)
    const isLargeFile = file.size > 10 * 1024 * 1024;

    // Para archivos pequeños, usar el método simple
    if (!isLargeFile) {
      // Convertir archivo a Base64
      const base64Content = await fileToBase64(file);

      // Crear documento con el contenido binario
      const documentData = {
        filename: file.name,
        mimetype: file.type,
        size: file.size,
        binary: base64Content,
        ...metadata,
      };

      // Crear documento en la colección
      const result = await makeApiRequest(
        `${API_URL}/collections/${collection}`,
        {
          method: "POST",
          body: JSON.stringify(documentData),
          useCache: false,
        }
      );

      return result;
    }

    // Para archivos grandes, usar streaming por fragmentos
    // Crear un ID temporal para el archivo
    const tempId = `temp_${Date.now()}_${Math.random()
      .toString(36)
      .substring(2, 15)}`;

    // Crear documento inicial con metadatos pero sin contenido
    const initialData = {
      filename: file.name,
      mimetype: file.type,
      size: file.size,
      chunks: Math.ceil(file.size / CHUNK_SIZE),
      tempId: tempId,
      isMultipart: true,
      chunkSize: CHUNK_SIZE,
      completedChunks: 0,
      ...metadata,
    };

    // Crear documento inicial
    const initialResult = await makeApiRequest(
      `${API_URL}/collections/${collection}`,
      {
        method: "POST",
        body: JSON.stringify(initialData),
        useCache: false,
      }
    );

    const docId = initialResult.id;
    const totalChunks = Math.ceil(file.size / CHUNK_SIZE);

    // Subir fragmentos
    for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
      const start = chunkIndex * CHUNK_SIZE;
      const end = Math.min(start + CHUNK_SIZE, file.size);
      const chunk = file.slice(start, end);

      // Convertir fragmento a Base64
      const base64Chunk = await fileToBase64(chunk);

      // Calcular checksum del fragmento (para verificación de integridad)
      const chunkChecksum = await calculateSHA256(chunk);

      // Crear datos del fragmento
      const chunkData = {
        chunkIndex: chunkIndex,
        totalChunks: totalChunks,
        binary: base64Chunk,
        checksum: chunkChecksum,
        tempId: tempId,
      };

      // Subir fragmento
      await makeApiRequest(
        `${API_URL}/collections/${collection}/${docId}/chunk`,
        {
          method: "PUT",
          body: JSON.stringify(chunkData),
          useCache: false,
        }
      );

      // Actualizar progreso
      if (progressCallback) {
        const progress = Math.round(((chunkIndex + 1) / totalChunks) * 100);
        progressCallback(progress);
      }
    }

    // Finalizar la subida
    const finalizeData = {
      isComplete: true,
      tempId: tempId,
    };

    const finalResult = await makeApiRequest(
      `${API_URL}/collections/${collection}/${docId}/finalize`,
      {
        method: "PUT",
        body: JSON.stringify(finalizeData),
        useCache: false,
      }
    );

    return finalResult;
  } catch (error) {
    console.error("Error al subir archivo binario:", error);
    throw error;
  }
}

/**
 * Descarga un archivo binario de una colección
 * @param {string} collection - Nombre de la colección
 * @param {string} id - ID del documento
 * @param {Function} [progressCallback] - Función de callback para reportar progreso (0-100)
 * @returns {Promise<Object>} - Promesa que resuelve con el objeto {blob, filename, mimetype}
 */
async function downloadBinaryFromCollection(
  collection,
  id,
  progressCallback = null
) {
  try {
    // Obtener documento (metadatos)
    const doc = await makeApiRequest(
      `${API_URL}/collections/${collection}/${id}`
    );

    if (!doc || !doc.data) {
      throw new Error("No se pudo obtener el documento");
    }

    // Verificar si es un archivo multipart (grande)
    const isMultipart = doc.data.isMultipart === true;

    // Para archivos pequeños, usar el método simple
    if (!isMultipart) {
      if (!doc.data.binary) {
        throw new Error("El documento no contiene datos binarios");
      }

      // Convertir Base64 a Blob
      const byteCharacters = atob(doc.data.binary);
      const byteArrays = [];

      for (let offset = 0; offset < byteCharacters.length; offset += 512) {
        const slice = byteCharacters.slice(offset, offset + 512);

        const byteNumbers = new Array(slice.length);
        for (let i = 0; i < slice.length; i++) {
          byteNumbers[i] = slice.charCodeAt(i);
        }

        const byteArray = new Uint8Array(byteNumbers);
        byteArrays.push(byteArray);
      }

      // Crear Blob con el tipo MIME correcto
      const blob = new Blob(byteArrays, {
        type: doc.data.mimetype || "application/octet-stream",
      });
      return {
        blob,
        filename: doc.data.filename || `file_${id}`,
        mimetype: doc.data.mimetype || "application/octet-stream",
      };
    }

    // Para archivos grandes, descargar por fragmentos
    const totalChunks = doc.data.chunks || 0;
    if (totalChunks <= 0) {
      throw new Error("Información de fragmentos no válida");
    }

    // Array para almacenar todos los fragmentos
    const chunks = [];

    // Descargar cada fragmento
    for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
      // Solicitar fragmento
      const chunkResponse = await makeApiRequest(
        `${API_URL}/collections/${collection}/${id}/chunk/${chunkIndex}`,
        { useCache: false }
      );

      if (!chunkResponse || !chunkResponse.binary) {
        throw new Error(`Error al descargar fragmento ${chunkIndex}`);
      }

      // Verificar checksum si está disponible
      if (chunkResponse.checksum) {
        const chunkData = atob(chunkResponse.binary);
        const byteNumbers = new Array(chunkData.length);
        for (let i = 0; i < chunkData.length; i++) {
          byteNumbers[i] = chunkData.charCodeAt(i);
        }
        const byteArray = new Uint8Array(byteNumbers);
        const blob = new Blob([byteArray]);

        // Calcular checksum del fragmento recibido
        const calculatedChecksum = await calculateSHA256(blob);

        // Verificar integridad
        if (calculatedChecksum !== chunkResponse.checksum) {
          throw new Error(
            `Error de integridad en fragmento ${chunkIndex}: checksum no coincide`
          );
        }
      }

      // Convertir Base64 a ArrayBuffer
      const byteCharacters = atob(chunkResponse.binary);
      const byteNumbers = new Array(byteCharacters.length);
      for (let i = 0; i < byteCharacters.length; i++) {
        byteNumbers[i] = byteCharacters.charCodeAt(i);
      }
      const byteArray = new Uint8Array(byteNumbers);

      // Guardar fragmento
      chunks.push(byteArray);

      // Actualizar progreso
      if (progressCallback) {
        const progress = Math.round(((chunkIndex + 1) / totalChunks) * 100);
        progressCallback(progress);
      }
    }

    // Combinar todos los fragmentos en un solo Blob
    const blob = new Blob(chunks, {
      type: doc.data.mimetype || "application/octet-stream",
    });

    return {
      blob,
      filename: doc.data.filename || `file_${id}`,
      mimetype: doc.data.mimetype || "application/octet-stream",
    };
  } catch (error) {
    console.error("Error al descargar archivo binario:", error);
    throw error;
  }
}

/**
 * Descarga y abre un archivo binario
 * @param {string} collection - Nombre de la colección
 * @param {string} id - ID del documento
 * @param {boolean} [visualize=false] - Si es true, intenta abrir el archivo en el navegador; si es false, lo descarga
 */
async function openBinaryFile(collection, id, visualize = false) {
  try {
    console.log(
      `Iniciando descarga de archivo binario: colección=${collection}, id=${id}`
    );
    showLoading();

    // Crear un elemento de progreso
    const progressContainer = document.createElement("div");
    progressContainer.className =
      "progress-container position-fixed top-50 start-50 translate-middle p-3 bg-white rounded shadow-lg";
    progressContainer.style.zIndex = "9999";
    progressContainer.style.width = "300px";

    const progressText = document.createElement("div");
    progressText.className = "text-center mb-2";
    progressText.textContent = "Descargando archivo...";

    const progressBar = document.createElement("div");
    progressBar.className = "progress";
    progressBar.innerHTML = `<div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" style="width: 0%" aria-valuenow="0" aria-valuemin="0" aria-valuemax="100">0%</div>`;

    progressContainer.appendChild(progressText);
    progressContainer.appendChild(progressBar);

    // Función para actualizar el progreso
    const updateProgress = (percent) => {
      const progressBarInner = progressBar.querySelector(".progress-bar");
      progressBarInner.style.width = `${percent}%`;
      progressBarInner.setAttribute("aria-valuenow", percent);
      progressBarInner.textContent = `${percent}%`;
      console.log(`Progreso de descarga: ${percent}%`);
    };

    // Mostrar el progreso solo para archivos grandes
    console.log(`Obteniendo metadatos del documento: ${collection}/${id}`);
    const doc = await makeApiRequest(
      `${API_URL}/collections/${collection}/${id}`
    );
    console.log(`Metadatos recibidos:`, doc);

    const isLargeFile = doc && doc.data && doc.data.isMultipart === true;
    console.log(`¿Es archivo grande? ${isLargeFile}`);

    if (isLargeFile) {
      document.body.appendChild(progressContainer);
    }

    // Descargar el archivo con progreso
    console.log(`Iniciando descarga del contenido...`);
    const { blob, filename, mimetype } = await downloadBinaryFromCollection(
      collection,
      id,
      isLargeFile ? updateProgress : null
    );
    console.log(
      `Descarga completada: ${filename} (${mimetype}, ${blob.size} bytes)`
    );

    // Eliminar el indicador de progreso
    if (isLargeFile && progressContainer.parentNode) {
      document.body.removeChild(progressContainer);
    }

    // Crear URL para el blob
    const url = URL.createObjectURL(blob);
    console.log(`URL del blob creada: ${url}`);

    // Verificar si el navegador puede manejar el tipo de archivo
    const canHandle =
      mimetype &&
      (mimetype.startsWith("image/") ||
        mimetype.startsWith("text/") ||
        mimetype === "application/pdf");

    // Decidir si visualizar o descargar basado en el parámetro y el tipo de archivo
    if (visualize && canHandle) {
      console.log(`Abriendo archivo ${filename} en el navegador`);
      // Abrir en una nueva pestaña
      window.open(url, "_blank");

      // Limpiar después de un tiempo
      setTimeout(() => {
        URL.revokeObjectURL(url);
      }, 1000);

      showAlert(`Archivo ${filename} abierto en una nueva pestaña`, "success");
    } else {
      if (visualize && !canHandle) {
        console.log(
          `El navegador no puede manejar el tipo ${mimetype}, descargando en su lugar`
        );
        showAlert(
          `El tipo de archivo ${mimetype} no se puede visualizar directamente. Se descargará el archivo.`,
          "warning"
        );
      }

      console.log(`Descargando archivo ${filename}`);
      // Descargar el archivo
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      a.style.display = "none";
      document.body.appendChild(a);

      // Simular clic
      a.click();

      // Limpiar
      setTimeout(() => {
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      }, 1000);

      showAlert(`Archivo ${filename} descargado correctamente`, "success");
    }

    hideLoading();
  } catch (error) {
    console.error(`Error al descargar archivo:`, error);
    hideLoading();

    // Eliminar el indicador de progreso si existe
    const progressContainer = document.querySelector(".progress-container");
    if (progressContainer && progressContainer.parentNode) {
      document.body.removeChild(progressContainer);
    }

    showAlert(`Error al descargar archivo: ${error.message}`, "danger");
  }
}
