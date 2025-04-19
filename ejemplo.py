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
