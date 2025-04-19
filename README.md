# DBP2P - Base de Datos NoSQL Descentralizada P2P

DBP2P es una base de datos NoSQL descentralizada que utiliza tecnología peer-to-peer (P2P) para sincronizar datos entre nodos. Permite almacenar, consultar, actualizar y eliminar documentos JSON en colecciones, similar a MongoDB, pero con la ventaja de ser completamente descentralizada.

## Características

- **Descentralizada**: Los datos se sincronizan automáticamente entre todos los nodos conectados a la red P2P.
- **NoSQL**: Almacena documentos JSON con estructura flexible.
- **Colecciones**: Los documentos se organizan en colecciones (similar a MongoDB).
- **Operaciones CRUD**: Soporta operaciones Crear, Leer, Actualizar y Borrar.
- **Consultas avanzadas**: Sistema de consultas con operadores lógicos, comparaciones, expresiones regulares y ordenación.
- **Indexación**: Índices para mejorar el rendimiento de las consultas frecuentes.
- **Caché**: Sistema de caché para reducir el tiempo de acceso a documentos frecuentes.
- **Identificadores únicos**: Cada documento tiene un ID único generado automáticamente.
- **Persistencia**: Almacena los datos en disco para que no se pierdan al cerrar el programa.
- **Log de transacciones**: Registra todas las operaciones para recuperación en caso de fallos.
- **Copias de seguridad**: Permite crear y restaurar copias de seguridad de la base de datos.
- **Gestión de memoria**: Control automático del uso de memoria con compresión y limpieza.
- **Sincronización optimizada**: Sincronización incremental y completa entre nodos con priorización.
- **Sistema de roles**: Control de acceso basado en roles para gestionar permisos de usuarios.
- **API REST**: Interfaz REST con autenticación por tokens JWT para acceso programático.
- **WebSockets**: Actualizaciones en tiempo real para suscribirse a cambios en la base de datos.
- **Cifrado**: Comunicación cifrada entre nodos para mayor seguridad.
- **Almacenamiento binario**: Soporte para almacenar y recuperar datos binarios con compresión.

## Requisitos

- Go 1.24.2 o superior

## Instalación

### Desde el código fuente

1. Clona el repositorio:
   ```
   git clone https://github.com/aratan/dbp2p.git
   cd dbp2p
   ```

2. Instala las dependencias:
   ```
   go get github.com/libp2p/go-libp2p github.com/libp2p/go-libp2p-pubsub
   ```

3. Compila el proyecto:
   ```
   go build
   ```

### Usando el binario precompilado

Simplemente descarga el archivo `dbp2p.exe` (Windows) o `dbp2p` (Linux/Mac) desde la sección de releases y ejecútalo.

## Uso

DBP2P puede ejecutarse en diferentes modos:

### Modo CLI (Interfaz de Línea de Comandos)

Para iniciar la base de datos en modo CLI:

```
./dbp2p
```

o en Windows:

```
.\dbp2p.exe
```

### Modo Servidor (API REST y WebSocket)

Para iniciar la base de datos como servidor con API REST y WebSocket:

```
./dbp2p --api=true --ws=true --api-port=8080 --ws-port=8081
```

Opciones disponibles:

- `--api`: Habilitar API REST (por defecto: true)
- `--ws`: Habilitar WebSocket (por defecto: true)
- `--api-port`: Puerto para la API REST (por defecto: 8080)
- `--ws-port`: Puerto para WebSocket (por defecto: 8081)

## Guía de uso por interfaz

### 1. Interfaz de Línea de Comandos (CLI)

La CLI proporciona una forma interactiva de trabajar con la base de datos. Aquí hay ejemplos de los comandos más comunes:

#### Crear un documento

```
> create usuarios {"nombre": "Juan García", "edad": 30, "email": "juan@ejemplo.com", "intereses": ["programación", "música", "viajes"]}
Documento creado con ID: 9c1612c6-5393-48ca-85a7-450500e999aa
```

#### Obtener un documento por ID

```
> get 9c1612c6-5393-48ca-85a7-450500e999aa
{
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 30,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:03:04.3571741+02:00"
}
```

#### Buscar documentos por criterios

```
> query usuarios {"edad": 30}
Encontrados 1 documentos:

[1] {
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 30,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:03:04.3571741+02:00"
}
```

#### Actualizar un documento

```
> update 9c1612c6-5393-48ca-85a7-450500e999aa {"edad": 31, "telefono": "123456789"}
Documento actualizado:
{
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 31,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García",
    "telefono": "123456789"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:04:12.0260447+02:00"
}
```

#### Eliminar un documento

```
> delete 9c1612c6-5393-48ca-85a7-450500e999aa
Documento con ID 9c1612c6-5393-48ca-85a7-450500e999aa eliminado
```

#### Listar todos los documentos de una colección

```
> list usuarios
Encontrados 2 documentos en la colección 'usuarios':

[1] ID: 9c1612c6-5393-48ca-85a7-450500e999aa
    Datos: {
      "edad": 31,
      "email": "juan@ejemplo.com",
      "intereses": [
        "programación",
        "música",
        "viajes"
      ],
      "nombre": "Juan García",
      "telefono": "123456789"
    }

[2] ID: 76eed9d1-f065-4919-a462-a4c4a2426205
    Datos: {
      "edad": 28,
      "email": "maria@ejemplo.com",
      "nombre": "María López",
      "profesion": "Ingeniera de Software"
    }
```

#### Crear una copia de seguridad

```
> backup
Copia de seguridad creada: backup_20250412_150405
```

#### Restaurar desde una copia de seguridad

```
> restore backup_20250412_150405
Base de datos restaurada desde: backup_20250412_150405
```

#### Listar copias de seguridad disponibles

```
> list_backups
Copias de seguridad disponibles:
  [1] backup_20250412_150405
  [2] backup_20250412_143022
```

### Comandos disponibles

Una vez que el programa esté en ejecución, verás una interfaz de línea de comandos con los siguientes comandos disponibles:

#### 1. Crear un documento

```
create <colección> <json_data>
```

**Ejemplo:**
```
create usuarios {"nombre": "Juan García", "edad": 30, "email": "juan@ejemplo.com", "intereses": ["programación", "música", "viajes"]}
```

**Respuesta:**
```
Documento creado con ID: 9c1612c6-5393-48ca-85a7-450500e999aa
```

#### 2. Obtener un documento por ID

```
get <id>
```

**Ejemplo:**
```
get 9c1612c6-5393-48ca-85a7-450500e999aa
```

**Respuesta:**
```
{
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 30,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:03:04.3571741+02:00"
}
```

#### 3. Buscar documentos (consulta por criterios)

```
query <colección> <json_query>
```

**Ejemplo:**
```
query usuarios {"edad": 30}
```

**Respuesta:**
```
Encontrados 1 documentos:

[1] {
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 30,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:03:04.3571741+02:00"
}
```

#### 4. Actualizar un documento

```
update <id> <json_data>
```

**Ejemplo:**
```
update 9c1612c6-5393-48ca-85a7-450500e999aa {"edad": 31, "telefono": "123456789"}
```

**Respuesta:**
```
Documento actualizado:
{
  "id": "9c1612c6-5393-48ca-85a7-450500e999aa",
  "collection": "usuarios",
  "data": {
    "edad": 31,
    "email": "juan@ejemplo.com",
    "intereses": [
      "programación",
      "música",
      "viajes"
    ],
    "nombre": "Juan García",
    "telefono": "123456789"
  },
  "created_at": "2025-04-12T13:03:04.3571741+02:00",
  "updated_at": "2025-04-12T13:04:12.0260447+02:00"
}
```

#### 5. Eliminar un documento

```
delete <id>
```

**Ejemplo:**
```
delete 9c1612c6-5393-48ca-85a7-450500e999aa
```

**Respuesta:**
```
Documento con ID 9c1612c6-5393-48ca-85a7-450500e999aa eliminado
```

#### 6. Listar todos los documentos de una colección

```
list <colección>
```

**Ejemplo:**
```
list usuarios
```

**Respuesta:**
```
Encontrados 2 documentos en la colección 'usuarios':

[1] ID: 9c1612c6-5393-48ca-85a7-450500e999aa
    Datos: {
      "edad": 31,
      "email": "juan@ejemplo.com",
      "intereses": [
        "programación",
        "música",
        "viajes"
      ],
      "nombre": "Juan García",
      "telefono": "123456789"
    }

[2] ID: 76eed9d1-f065-4919-a462-a4c4a2426205
    Datos: {
      "edad": 28,
      "email": "maria@ejemplo.com",
      "nombre": "María López",
      "profesion": "Ingeniera de Software"
    }
```

#### 7. Crear una copia de seguridad

```
backup
```

**Ejemplo:**
```
backup
```

**Respuesta:**
```
Copia de seguridad creada: backup_20250412_150405
```

#### 8. Restaurar desde una copia de seguridad

```
restore <nombre_backup>
```

**Ejemplo:**
```
restore backup_20250412_150405
```

**Respuesta:**
```
Base de datos restaurada desde: backup_20250412_150405
```

#### 9. Listar copias de seguridad disponibles

```
list_backups
```

**Ejemplo:**
```
list_backups
```

**Respuesta:**
```
Copias de seguridad disponibles:
  [1] backup_20250412_150405
  [2] backup_20250412_143022
```

#### 10. Salir del programa

```
exit
```

### 2. API REST

La API REST proporciona acceso programático a la base de datos. Todas las rutas requieren autenticación mediante token JWT o clave API.

#### Autenticación

Para obtener un token JWT, primero debes autenticarte con tus credenciales:

```bash
# Usando curl (Linux/Mac)
curl -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' http://localhost:8080/api/login

# Usando PowerShell (Windows)
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/login" -ContentType "application/json" -Body '{"username":"admin","password":"admin123"}'
```

Respuesta:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "admin",
  "roles": "admin"
}
```

Usa el token en las solicitudes posteriores en el encabezado `Authorization`:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

#### Ejemplos de operaciones CRUD

##### Crear un documento

```bash
# Usando curl (Linux/Mac)
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer TU_TOKEN_JWT" -d '{"nombre":"Juan García","edad":30,"email":"juan@ejemplo.com"}' http://localhost:8080/api/collections/usuarios

# Usando PowerShell (Windows)
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/collections/usuarios" -ContentType "application/json" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"} -Body '{"nombre":"Juan García","edad":30,"email":"juan@ejemplo.com"}'
```

##### Obtener todos los documentos de una colección

```bash
# Usando curl (Linux/Mac)
curl -H "Authorization: Bearer TU_TOKEN_JWT" http://localhost:8080/api/collections/usuarios

# Usando PowerShell (Windows)
Invoke-RestMethod -Uri "http://localhost:8080/api/collections/usuarios" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"}
```

##### Obtener un documento específico

```bash
# Usando curl (Linux/Mac)
curl -H "Authorization: Bearer TU_TOKEN_JWT" http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa

# Usando PowerShell (Windows)
Invoke-RestMethod -Uri "http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"}
```

##### Actualizar un documento

```bash
# Usando curl (Linux/Mac)
curl -X PUT -H "Content-Type: application/json" -H "Authorization: Bearer TU_TOKEN_JWT" -d '{"edad":31,"telefono":"123456789"}' http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa

# Usando PowerShell (Windows)
Invoke-RestMethod -Method PUT -Uri "http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa" -ContentType "application/json" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"} -Body '{"edad":31,"telefono":"123456789"}'
```

##### Eliminar un documento

```bash
# Usando curl (Linux/Mac)
curl -X DELETE -H "Authorization: Bearer TU_TOKEN_JWT" http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa

# Usando PowerShell (Windows)
Invoke-RestMethod -Method DELETE -Uri "http://localhost:8080/api/collections/usuarios/9c1612c6-5393-48ca-85a7-450500e999aa" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"}
```

#### Gestión de usuarios y roles

##### Crear un nuevo usuario

```bash
# Usando curl (Linux/Mac)
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer TU_TOKEN_JWT" -d '{"username":"nuevo_usuario","password":"contraseña","roles":["reader","writer"]}' http://localhost:8080/api/users

# Usando PowerShell (Windows)
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/users" -ContentType "application/json" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"} -Body '{"username":"nuevo_usuario","password":"contraseña","roles":["reader","writer"]}'
```

##### Crear una clave API para un usuario

```bash
# Usando curl (Linux/Mac)
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer TU_TOKEN_JWT" -d '{"name":"Clave para aplicación web","valid_days":30}' http://localhost:8080/api/users/550e8400-e29b-41d4-a716-446655440000/apikeys

# Usando PowerShell (Windows)
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/users/550e8400-e29b-41d4-a716-446655440000/apikeys" -ContentType "application/json" -Headers @{"Authorization"="Bearer TU_TOKEN_JWT"} -Body '{"name":"Clave para aplicación web","valid_days":30}'
```

### 2.1 Python

```python
#!/usr/bin/env python
"""
Cliente Python para DBP2P - Base de datos NoSQL descentralizada P2P
Este script demuestra todas las funcionalidades de la API REST y WebSocket
"""

import argparse
import json
import requests
import time
import websocket
import threading
import sys
from datetime import datetime

# Configuración por defecto
DEFAULT_API_URL = "http://localhost:8080"
DEFAULT_WS_URL = "ws://localhost:8081"

# Colores para la terminal
class Colors:
    HEADER = '\033[95m'
    BLUE = '\033[94m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

class DBP2PClient:
    def __init__(self, api_url=DEFAULT_API_URL, ws_url=DEFAULT_WS_URL):
        self.api_url = api_url
        self.ws_url = ws_url
        self.token = None
        self.user_id = None
        self.username = None
        self.roles = None
        self.ws = None
        self.ws_connected = False
        self.subscriptions = []

    def login(self, username, password):
        """Iniciar sesión y obtener token JWT"""
        print(f"{Colors.BLUE}Iniciando sesión como {username}...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/login",
                json={"username": username, "password": password}
            )
            response.raise_for_status()
            data = response.json()
            self.token = data.get("token")
            self.user_id = data.get("user_id")
            self.username = data.get("username")
            self.roles = data.get("roles", "").split(",")
            print(f"{Colors.GREEN}Sesión iniciada correctamente como {self.username} (roles: {', '.join(self.roles)}){Colors.ENDC}")
            return True
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al iniciar sesión: {str(e)}{Colors.ENDC}")
            return False

    def get_headers(self):
        """Obtener headers con token de autenticación"""
        if not self.token:
            raise ValueError("No hay sesión iniciada. Usa login() primero.")
        return {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json"
        }

    def create_document(self, collection, data):
        """Crear un nuevo documento"""
        print(f"{Colors.BLUE}Creando documento en colección '{collection}'...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/collections/{collection}",
                headers=self.get_headers(),
                json=data
            )
            response.raise_for_status()
            doc = response.json()
            print(f"{Colors.GREEN}Documento creado con ID: {doc.get('id')}{Colors.ENDC}")
            return doc
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al crear documento: {str(e)}{Colors.ENDC}")
            return None

    def get_document(self, collection, doc_id):
        """Obtener un documento por ID"""
        print(f"{Colors.BLUE}Obteniendo documento {doc_id} de colección '{collection}'...{Colors.ENDC}")
        try:
            response = requests.get(
                f"{self.api_url}/api/collections/{collection}/{doc_id}",
                headers=self.get_headers()
            )
            response.raise_for_status()
            doc = response.json()
            print(f"{Colors.GREEN}Documento obtenido:{Colors.ENDC}")
            print(json.dumps(doc, indent=2))
            return doc
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al obtener documento: {str(e)}{Colors.ENDC}")
            return None

    def update_document(self, collection, doc_id, data):
        """Actualizar un documento"""
        print(f"{Colors.BLUE}Actualizando documento {doc_id} en colección '{collection}'...{Colors.ENDC}")
        try:
            response = requests.put(
                f"{self.api_url}/api/collections/{collection}/{doc_id}",
                headers=self.get_headers(),
                json=data
            )
            response.raise_for_status()
            doc = response.json()
            print(f"{Colors.GREEN}Documento actualizado:{Colors.ENDC}")
            print(json.dumps(doc, indent=2))
            return doc
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al actualizar documento: {str(e)}{Colors.ENDC}")
            return None

    def delete_document(self, collection, doc_id):
        """Eliminar un documento"""
        print(f"{Colors.BLUE}Eliminando documento {doc_id} de colección '{collection}'...{Colors.ENDC}")
        try:
            response = requests.delete(
                f"{self.api_url}/api/collections/{collection}/{doc_id}",
                headers=self.get_headers()
            )
            response.raise_for_status()
            print(f"{Colors.GREEN}Documento eliminado correctamente{Colors.ENDC}")
            return True
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al eliminar documento: {str(e)}{Colors.ENDC}")
            return False

    def list_collection(self, collection):
        """Listar todos los documentos de una colección"""
        print(f"{Colors.BLUE}Listando documentos de colección '{collection}'...{Colors.ENDC}")
        try:
            response = requests.get(
                f"{self.api_url}/api/collections/{collection}",
                headers=self.get_headers()
            )
            response.raise_for_status()
            docs = response.json()
            if docs is None:
                docs = []
            print(f"{Colors.GREEN}Encontrados {len(docs)} documentos:{Colors.ENDC}")
            for i, doc in enumerate(docs, 1):
                print(f"[{i}] ID: {doc.get('id')}")
                print(f"    Datos: {json.dumps(doc.get('data'), indent=2)}")
            return docs
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al listar colección: {str(e)}{Colors.ENDC}")
            return []

    def create_backup(self):
        """Crear una copia de seguridad"""
        print(f"{Colors.BLUE}Creando copia de seguridad...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/backups",
                headers=self.get_headers()
            )
            response.raise_for_status()
            data = response.json()
            backup_name = data.get("backup_name")
            print(f"{Colors.GREEN}Copia de seguridad creada: {backup_name}{Colors.ENDC}")
            return backup_name
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al crear copia de seguridad: {str(e)}{Colors.ENDC}")
            return None

    def list_backups(self):
        """Listar copias de seguridad disponibles"""
        print(f"{Colors.BLUE}Listando copias de seguridad...{Colors.ENDC}")
        try:
            response = requests.get(
                f"{self.api_url}/api/backups",
                headers=self.get_headers()
            )
            response.raise_for_status()
            backups = response.json()
            if backups is None:
                backups = []
            print(f"{Colors.GREEN}Copias de seguridad disponibles:{Colors.ENDC}")
            for i, backup in enumerate(backups, 1):
                print(f"  [{i}] {backup}")
            return backups
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al listar copias de seguridad: {str(e)}{Colors.ENDC}")
            return []

    def restore_backup(self, backup_name):
        """Restaurar desde una copia de seguridad"""
        print(f"{Colors.BLUE}Restaurando desde copia de seguridad {backup_name}...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/backups/{backup_name}",
                headers=self.get_headers()
            )
            response.raise_for_status()
            print(f"{Colors.GREEN}Base de datos restaurada correctamente{Colors.ENDC}")
            return True
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al restaurar copia de seguridad: {str(e)}{Colors.ENDC}")
            return False

    def list_users(self):
        """Listar usuarios"""
        print(f"{Colors.BLUE}Listando usuarios...{Colors.ENDC}")
        try:
            response = requests.get(
                f"{self.api_url}/api/users",
                headers=self.get_headers()
            )
            response.raise_for_status()
            users = response.json()
            if users is None:
                users = []
            print(f"{Colors.GREEN}Usuarios registrados:{Colors.ENDC}")
            for i, user in enumerate(users, 1):
                print(f"[{i}] ID: {user.get('id')}")
                print(f"    Username: {user.get('username')}")
                print(f"    Roles: {', '.join(user.get('roles', []))}")
                print(f"    API Keys: {len(user.get('api_keys', []))}")
            return users
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al listar usuarios: {str(e)}{Colors.ENDC}")
            return []

    def create_user(self, username, password, roles):
        """Crear un nuevo usuario"""
        print(f"{Colors.BLUE}Creando usuario {username}...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/users",
                headers=self.get_headers(),
                json={"username": username, "password": password, "roles": roles}
            )
            response.raise_for_status()
            user = response.json()
            print(f"{Colors.GREEN}Usuario creado con ID: {user.get('id')}{Colors.ENDC}")
            return user
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al crear usuario: {str(e)}{Colors.ENDC}")
            return None

    def create_api_key(self, user_id, name, valid_days):
        """Crear una clave API para un usuario"""
        print(f"{Colors.BLUE}Creando clave API para usuario {user_id}...{Colors.ENDC}")
        try:
            response = requests.post(
                f"{self.api_url}/api/users/{user_id}/apikeys",
                headers=self.get_headers(),
                json={"name": name, "valid_days": valid_days}
            )
            response.raise_for_status()
            api_key = response.json()
            print(f"{Colors.GREEN}Clave API creada: {api_key.get('token')}{Colors.ENDC}")
            return api_key
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al crear clave API: {str(e)}{Colors.ENDC}")
            return None

    def list_roles(self):
        """Listar roles"""
        print(f"{Colors.BLUE}Listando roles...{Colors.ENDC}")
        try:
            response = requests.get(
                f"{self.api_url}/api/roles",
                headers=self.get_headers()
            )
            response.raise_for_status()
            roles = response.json()
            if roles is None:
                roles = []
            print(f"{Colors.GREEN}Roles disponibles:{Colors.ENDC}")
            for i, role in enumerate(roles, 1):
                print(f"[{i}] Nombre: {role.get('name')}")
                print(f"    Descripción: {role.get('description')}")
                print(f"    Permisos: {len(role.get('permissions', []))}")
            return roles
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al listar roles: {str(e)}{Colors.ENDC}")
            return []

    # WebSocket functions
    def on_ws_message(self, ws, message):
        """Callback para mensajes WebSocket"""
        try:
            data = json.loads(message)
            timestamp = datetime.now().strftime("%H:%M:%S")
            print(f"\n{Colors.YELLOW}[{timestamp}] Evento WebSocket recibido:{Colors.ENDC}")

            if "type" in data:
                event_type = data.get("type")
                collection = data.get("collection")
                doc_id = data.get("document_id")

                if event_type == "create":
                    print(f"{Colors.YELLOW}Documento creado en '{collection}' con ID: {doc_id}{Colors.ENDC}")
                elif event_type == "update":
                    print(f"{Colors.YELLOW}Documento actualizado en '{collection}' con ID: {doc_id}{Colors.ENDC}")
                elif event_type == "delete":
                    print(f"{Colors.YELLOW}Documento eliminado de '{collection}' con ID: {doc_id}{Colors.ENDC}")

                if "document" in data and data["document"]:
                    print(f"{Colors.YELLOW}Datos: {json.dumps(data['document'].get('data', {}), indent=2)}{Colors.ENDC}")
            else:
                print(f"{Colors.YELLOW}Mensaje: {message}{Colors.ENDC}")
        except json.JSONDecodeError:
            print(f"{Colors.YELLOW}Mensaje no JSON: {message}{Colors.ENDC}")

    def on_ws_error(self, ws, error):
        """Callback para errores WebSocket"""
        print(f"{Colors.RED}Error WebSocket: {error}{Colors.ENDC}")

    def on_ws_close(self, ws, close_status_code, close_msg):
        """Callback para cierre de WebSocket"""
        self.ws_connected = False
        print(f"{Colors.RED}Conexión WebSocket cerrada{Colors.ENDC}")

    def on_ws_open(self, ws):
        """Callback para apertura de WebSocket"""
        self.ws_connected = True
        print(f"{Colors.GREEN}Conexión WebSocket establecida{Colors.ENDC}")

        # Restaurar suscripciones
        for sub in self.subscriptions:
            self.subscribe(sub["collection"], sub["document_id"])

    def connect_websocket(self):
        """Conectar al WebSocket"""
        if not self.token:
            raise ValueError("No hay sesión iniciada. Usa login() primero.")

        print(f"{Colors.BLUE}Conectando a WebSocket...{Colors.ENDC}")
        ws_app = websocket.WebSocketApp(
            f"{self.ws_url}/ws?token={self.token}",
            on_message=self.on_ws_message,
            on_error=self.on_ws_error,
            on_close=self.on_ws_close,
            on_open=self.on_ws_open
        )

        self.ws = ws_app

        # Iniciar WebSocket en un hilo separado
        ws_thread = threading.Thread(target=ws_app.run_forever)
        ws_thread.daemon = True
        ws_thread.start()

        # Esperar a que se establezca la conexión
        timeout = 5
        start_time = time.time()
        while not self.ws_connected and time.time() - start_time < timeout:
            time.sleep(0.1)

        if not self.ws_connected:
            print(f"{Colors.RED}No se pudo establecer la conexión WebSocket{Colors.ENDC}")
            return False

        return True

    def subscribe(self, collection, document_id=""):
        """Suscribirse a eventos de una colección o documento"""
        if not self.ws or not self.ws_connected:
            print(f"{Colors.RED}No hay conexión WebSocket. Usa connect_websocket() primero.{Colors.ENDC}")
            return False

        subscription = {
            "action": "subscribe",
            "collection": collection,
            "document_id": document_id
        }

        print(f"{Colors.BLUE}Suscribiéndose a {collection}{' documento ' + document_id if document_id else ''}...{Colors.ENDC}")
        self.ws.send(json.dumps(subscription))

        # Guardar suscripción
        self.subscriptions.append({"collection": collection, "document_id": document_id})

        return True

    def unsubscribe(self, collection, document_id=""):
        """Cancelar suscripción a eventos"""
        if not self.ws or not self.ws_connected:
            print(f"{Colors.RED}No hay conexión WebSocket. Usa connect_websocket() primero.{Colors.ENDC}")
            return False

        unsubscription = {
            "action": "unsubscribe",
            "collection": collection,
            "document_id": document_id
        }

        print(f"{Colors.BLUE}Cancelando suscripción a {collection}{' documento ' + document_id if document_id else ''}...{Colors.ENDC}")
        self.ws.send(json.dumps(unsubscription))

        # Eliminar suscripción
        self.subscriptions = [s for s in self.subscriptions
                             if not (s["collection"] == collection and s["document_id"] == document_id)]

        return True

def demo_crud_operations(client):
    """Demostración de operaciones CRUD"""
    print(f"\n{Colors.HEADER}=== DEMOSTRACIÓN DE OPERACIONES CRUD ==={Colors.ENDC}")

    # Crear un documento
    doc = client.create_document("usuarios", {
        "nombre": "Juan García",
        "edad": 30,
        "email": "juan@ejemplo.com",
        "intereses": ["programación", "música", "viajes"]
    })

    if not doc:
        return

    doc_id = doc.get("id")

    # Obtener el documento
    client.get_document("usuarios", doc_id)

    # Listar la colección
    client.list_collection("usuarios")

    # Actualizar el documento
    client.update_document("usuarios", doc_id, {
        "edad": 31,
        "telefono": "123456789"
    })

    # Obtener el documento actualizado
    client.get_document("usuarios", doc_id)

    # Eliminar el documento
    client.delete_document("usuarios", doc_id)

    # Verificar que se eliminó
    client.list_collection("usuarios")

def demo_backup_operations(client):
    """Demostración de operaciones de copia de seguridad"""
    print(f"\n{Colors.HEADER}=== DEMOSTRACIÓN DE COPIAS DE SEGURIDAD ==={Colors.ENDC}")

    # Crear un documento para la demostración
    doc = client.create_document("productos", {
        "nombre": "Laptop",
        "precio": 1200,
        "stock": 10
    })

    if not doc:
        print(f"{Colors.RED}No se pudo crear el documento de prueba. Saltando demostración de backup.{Colors.ENDC}")
        return

    # Listar la colección
    client.list_collection("productos")

    # Crear una copia de seguridad
    backup_name = client.create_backup()

    if not backup_name:
        print(f"{Colors.RED}No se pudo crear la copia de seguridad. Saltando resto de la demostración.{Colors.ENDC}")
        return

    # Listar copias de seguridad
    backups = client.list_backups()
    if not backups:
        print(f"{Colors.YELLOW}No se encontraron copias de seguridad.{Colors.ENDC}")

    # Eliminar el documento
    client.delete_document("productos", doc.get("id"))

    # Verificar que se eliminó
    client.list_collection("productos")

    # Restaurar desde la copia de seguridad
    if not client.restore_backup(backup_name):
        print(f"{Colors.RED}No se pudo restaurar la copia de seguridad.{Colors.ENDC}")
        return

    # Verificar que se restauró
    client.list_collection("productos")

def demo_user_management(client):
    """Demostración de gestión de usuarios"""
    print(f"\n{Colors.HEADER}=== DEMOSTRACIÓN DE GESTIÓN DE USUARIOS ==={Colors.ENDC}")

    # Listar usuarios
    users = client.list_users()
    if not users:
        print(f"{Colors.YELLOW}No se encontraron usuarios.{Colors.ENDC}")

    # Listar roles
    roles = client.list_roles()
    if not roles:
        print(f"{Colors.YELLOW}No se encontraron roles.{Colors.ENDC}")

    # Crear un nuevo usuario
    user = client.create_user("test_user", "password123", ["reader"])

    if not user:
        print(f"{Colors.RED}No se pudo crear el usuario de prueba. Saltando resto de la demostración.{Colors.ENDC}")
        return

    # Listar usuarios para verificar
    client.list_users()

    # Crear una clave API para el usuario
    client.create_api_key(user.get("id"), "Clave de prueba", 30)

def demo_websocket(client):
    """Demostración de WebSocket"""
    print(f"\n{Colors.HEADER}=== DEMOSTRACIÓN DE WEBSOCKET ==={Colors.ENDC}")

    # Conectar al WebSocket
    if not client.connect_websocket():
        print(f"{Colors.RED}No se pudo conectar al WebSocket. Saltando demostración.{Colors.ENDC}")
        return

    # Suscribirse a la colección usuarios
    if not client.subscribe("usuarios"):
        print(f"{Colors.RED}No se pudo suscribir a la colección. Saltando demostración.{Colors.ENDC}")
        return

    print(f"{Colors.BLUE}Creando un documento para demostrar eventos en tiempo real...{Colors.ENDC}")

    # Crear un documento (debería generar un evento)
    doc = client.create_document("usuarios", {
        "nombre": "María López",
        "edad": 28,
        "email": "maria@ejemplo.com"
    })

    if not doc:
        print(f"{Colors.RED}No se pudo crear el documento de prueba. Saltando resto de la demostración.{Colors.ENDC}")
        return

    doc_id = doc.get("id")

    # Esperar un momento para ver el evento
    time.sleep(1)

    # Actualizar el documento (debería generar otro evento)
    if not client.update_document("usuarios", doc_id, {
        "profesion": "Ingeniera de Software"
    }):
        print(f"{Colors.YELLOW}No se pudo actualizar el documento.{Colors.ENDC}")

    # Esperar un momento para ver el evento
    time.sleep(1)

    # Eliminar el documento (debería generar otro evento)
    if not client.delete_document("usuarios", doc_id):
        print(f"{Colors.YELLOW}No se pudo eliminar el documento.{Colors.ENDC}")

    # Esperar un momento para ver el evento
    time.sleep(1)

    # Cancelar suscripción
    client.unsubscribe("usuarios")

    print(f"{Colors.GREEN}Demostración de WebSocket completada{Colors.ENDC}")

def main():
    parser = argparse.ArgumentParser(description="Cliente Python para DBP2P")
    parser.add_argument("--api-url", default=DEFAULT_API_URL, help=f"URL de la API REST (default: {DEFAULT_API_URL})")
    parser.add_argument("--ws-url", default=DEFAULT_WS_URL, help=f"URL del WebSocket (default: {DEFAULT_WS_URL})")
    parser.add_argument("--username", default="admin", help="Nombre de usuario (default: admin)")
    parser.add_argument("--password", default="admin123", help="Contraseña (default: admin123)")
    parser.add_argument("--demo", choices=["crud", "backup", "users", "websocket", "all"], default="all",
                        help="Demostración a ejecutar (default: all)")

    args = parser.parse_args()

    # Crear cliente
    client = DBP2PClient(args.api_url, args.ws_url)

    # Iniciar sesión
    if not client.login(args.username, args.password):
        sys.exit(1)

    # Ejecutar demostraciones
    if args.demo == "crud" or args.demo == "all":
        demo_crud_operations(client)

    if args.demo == "backup" or args.demo == "all":
        demo_backup_operations(client)

    if args.demo == "users" or args.demo == "all":
        demo_user_management(client)

    if args.demo == "websocket" or args.demo == "all":
        demo_websocket(client)

    print(f"\n{Colors.GREEN}Demostración completada{Colors.ENDC}")

if __name__ == "__main__":
    main()
```

### 3. WebSocket

La interfaz WebSocket permite recibir actualizaciones en tiempo real de la base de datos.

#### Conexión y suscripción a eventos

Aquí hay un ejemplo de cómo conectarse y suscribirse a eventos usando JavaScript:

```javascript
// Conectar al WebSocket con un token JWT
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."; // Tu token JWT
const ws = new WebSocket(`ws://localhost:8081/ws?token=${token}`);

// Manejar eventos de conexión
ws.onopen = () => {
  console.log("Conexión WebSocket establecida");

  // Suscribirse a una colección
  ws.send(JSON.stringify({
    action: "subscribe",
    collection: "usuarios",
    document_id: "" // Vacío para suscribirse a toda la colección
  }));
};

// Manejar mensajes recibidos
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Evento recibido:", data);

  // Ejemplo de manejo de diferentes tipos de eventos
  if (data.type === "create") {
    console.log("Nuevo documento creado:", data.document);
  } else if (data.type === "update") {
    console.log("Documento actualizado:", data.document);
  } else if (data.type === "delete") {
    console.log("Documento eliminado con ID:", data.document_id);
  }
};

// Manejar errores
ws.onerror = (error) => {
  console.error("Error en la conexión WebSocket:", error);
};

// Manejar cierre de conexión
ws.onclose = () => {
  console.log("Conexión WebSocket cerrada");
};

// Para cancelar una suscripción
function unsubscribe(collection, documentId = "") {
  ws.send(JSON.stringify({
    action: "unsubscribe",
    collection: collection,
    document_id: documentId
  }));
}

// Para cerrar la conexión cuando ya no se necesite
function closeConnection() {
  ws.close();
}
```

## Ejemplos de uso avanzado

### Documentos con estructuras anidadas

```
create productos {"nombre": "Laptop", "precio": 1200, "stock": 10, "caracteristicas": {"marca": "TechBrand", "modelo": "X200", "ram": "16GB"}}
```

### Consultas con múltiples criterios

```
query usuarios {"edad": 30, "nombre": "Juan García"}
```

### Actualización parcial de documentos

```
update 9c1612c6-5393-48ca-85a7-450500e999aa {"intereses": ["programación", "música", "viajes", "fotografía"]}
```

### Consultas avanzadas con operadores

```go
// Crear una consulta para usuarios activos ordenados por edad
query := db.NewQuery("users").
    Where("active", db.OperatorEQ, true).
    Sort("age", db.SortAscending)

// Ejecutar la consulta
results, err := query.Execute(database)
```

### Consultas con condiciones lógicas

```go
// Usuarios con edad entre 25 y 40
query := db.NewQuery("users").
    And(
        db.QueryCondition{Field: "age", Operator: db.OperatorGTE, Value: 25},
        db.QueryCondition{Field: "age", Operator: db.OperatorLTE, Value: 40},
    )

// Usuarios inactivos o con edad mayor a 50
query := db.NewQuery("users").
    Or(
        db.QueryCondition{Field: "active", Operator: db.OperatorEQ, Value: false},
        db.QueryCondition{Field: "age", Operator: db.OperatorGT, Value: 50},
    )
```

### Consultas con campos anidados

```go
// Usuarios con dirección en Madrid
query := db.NewQuery("users").
    Where("address.city", db.OperatorEQ, "Madrid")
```

### Gestión de memoria

```go
// Crear gestor de memoria con configuración personalizada
memoryManager := db.NewMemoryManager(database, db.MemoryManagerConfig{
    CheckInterval:     time.Second * 10,
    CleanupThreshold:  0.7,
    MaxDocuments:      1000,
    EnableCompression: true,
})

// Iniciar gestor de memoria
memoryManager.Start()

// Obtener estadísticas de memoria
stats := memoryManager.GetMemoryStats()
fmt.Printf("Documentos en memoria: %d\n", stats.DocumentCount)
fmt.Printf("Documentos comprimidos: %d\n", stats.CompressedDocs)
```

### Sincronización optimizada entre nodos

```go
// Crear gestor de sincronización con configuración personalizada
syncManager := p2p.NewSyncManager(node, database, p2p.SyncConfig{
    FullSyncInterval:        time.Hour * 24,
    IncrementalSyncInterval: time.Minute * 5,
    BatchSize:               100,
    UseCompression:          true,
    CompressionLevel:        6,
})

// Iniciar sincronización
syncManager.Start()

// Obtener estadísticas de sincronización
stats := syncManager.GetSyncStats()
fmt.Printf("Documentos enviados: %d\n", stats.DocumentsSent)
fmt.Printf("Documentos recibidos: %d\n", stats.DocumentsReceived)
```

### Almacenamiento y recuperación de datos binarios

```go
// Crear gestor de binarios
binaryManager, err := binary.NewBinaryManager(database, "./data/binaries")

// Almacenar un archivo
file, _ := os.Open("imagen.jpg")
metadata, err := binaryManager.StoreFile(
    file,
    "imagen.jpg",
    "image/jpeg",
    binary.WithTags([]string{"imagen", "perfil"}),
    binary.WithFileCompression(true),
)

// Recuperar un archivo
reader, meta, err := binaryManager.GetFile(metadata.ID)
outputFile, _ := os.Create("imagen_recuperada.jpg")
io.Copy(outputFile, reader)
```

## Funcionamiento en red

Para aprovechar la naturaleza descentralizada de esta base de datos:

1. Ejecuta el binario en diferentes máquinas dentro de la misma red.
2. Los nodos se descubrirán automáticamente y sincronizarán los datos.
3. Cualquier operación CRUD realizada en un nodo se propagará a los demás nodos.

## Sistema de roles y permisos

DBP2P implementa un sistema de control de acceso basado en roles (RBAC) que permite gestionar qué usuarios pueden realizar qué operaciones en la base de datos.

### Roles predefinidos

- **admin**: Acceso completo a todas las funciones y datos
- **reader**: Acceso de solo lectura a todos los datos
- **writer**: Acceso de lectura y escritura a todos los datos

### Permisos

Los permisos se definen por recurso (colección) y acción:

- **read**: Permiso para leer documentos
- **write**: Permiso para crear y actualizar documentos
- **delete**: Permiso para eliminar documentos
- **admin**: Permiso para realizar todas las operaciones

### Usuario administrador predeterminado

Al iniciar por primera vez, se crea un usuario administrador con las siguientes credenciales:

- **Usuario**: admin
- **Contraseña**: admin123

Se recomienda cambiar la contraseña inmediatamente después de la primera ejecución.

## Persistencia y recuperación

DBP2P implementa un sistema robusto de persistencia y recuperación que incluye:

### Persistencia en disco

Los documentos se almacenan en archivos JSON organizados por colecciones en el directorio `./data/collections/`. Cada documento se guarda como un archivo JSON individual con su ID como nombre de archivo.

### Log de transacciones

Todas las operaciones (crear, actualizar, eliminar) se registran en un archivo de log de transacciones (`./data/transactions.log`). Este log permite:

- Reconstruir el estado de la base de datos en caso de cierre inesperado
- Auditar los cambios realizados en la base de datos
- Recuperar datos en caso de corrupción

### Copias de seguridad y recuperación

DBP2P incluye funcionalidades para gestionar copias de seguridad:

- **Crear copias de seguridad**: Guarda el estado completo de la base de datos con un timestamp
- **Restaurar desde copias de seguridad**: Permite volver a un estado anterior de la base de datos
- **Listar copias de seguridad**: Muestra todas las copias de seguridad disponibles

Las copias de seguridad se almacenan en el directorio `./data/backups/` y contienen tanto los documentos como el log de transacciones.

## Limitaciones actuales

- **Autenticación**: No hay sistema de autenticación o autorización.
- **Sincronización**: La sincronización depende de que los nodos estén conectados en el momento de la operación.
- **Manejo de conflictos**: No hay un manejo avanzado de conflictos entre actualizaciones simultáneas.

## Arquitectura

DBP2P está construido sobre las siguientes tecnologías:

- **libp2p**: Para la comunicación peer-to-peer y descubrimiento de nodos.
- **GossipSub**: Para la publicación/suscripción de mensajes entre nodos.
- **AES-GCM**: Para el cifrado de mensajes.

La arquitectura consta de ocho componentes principales:

1. **Capa de base de datos**: Maneja el almacenamiento y consulta de documentos.
2. **Capa de persistencia**: Gestiona el almacenamiento en disco, log de transacciones y copias de seguridad.
3. **Capa de sincronización**: Gestiona la propagación de cambios entre nodos con sincronización optimizada.
4. **Capa de red P2P**: Maneja la comunicación entre nodos.
5. **Capa de autenticación y autorización**: Gestiona usuarios, roles y permisos.
6. **Capa de API**: Proporciona interfaces REST y WebSocket para acceso programático.
7. **Capa de gestión de memoria**: Controla el uso de memoria y realiza optimizaciones automáticas.
8. **Capa de almacenamiento binario**: Gestiona el almacenamiento y recuperación de datos binarios.

## Contribuir

Las contribuciones son bienvenidas. Por favor, siente libre de enviar pull requests o abrir issues para mejorar el proyecto.

## Licencia

Este proyecto está licenciado bajo la licencia MIT - ver el archivo LICENSE para más detalles.
