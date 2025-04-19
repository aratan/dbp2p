#!/usr/bin/env python
import requests
import sys

class Colors:
    GREEN = '\033[92m'
    RED = '\033[91m'
    ENDC = '\033[0m'

class DBP2PClient:
    def __init__(self, api_url="http://localhost:8080"):
        self.api_url = api_url
        self.token = None
        self.user_id = None
        self.username = None

    # MÉTODO LOGIN FALTANTE
    def login(self, username, password):
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
            print(f"{Colors.GREEN}Login exitoso como {self.username}{Colors.ENDC}")
            return True
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error de login: {str(e)}{Colors.ENDC}")
            return False

    # MÉTODO PARA LISTAR COLECCIÓN
    def list_collection(self, collection):
        try:
            response = requests.get(
                f"{self.api_url}/api/collections/{collection}",
                headers={"Authorization": f"Bearer {self.token}"}
            )
            response.raise_for_status()
            return response.json() or []
        except requests.exceptions.RequestException as e:
            print(f"{Colors.RED}Error al listar colección: {str(e)}{Colors.ENDC}")
            return []

def main():
    cliente = DBP2PClient()
    
    if not cliente.login(username="admin", password="admin123"):
        sys.exit(f"{Colors.RED}Error de autenticación{Colors.ENDC}")
    
    usuarios = cliente.list_collection("usuarios")
    
    if usuarios:
        print(f"\n{Colors.GREEN}Usuarios encontrados:{Colors.ENDC}")
        for usuario in usuarios:
            print(f"\nID: {usuario['id']}")
            print("Datos:")
            for key, value in usuario.get('data', {}).items():
                print(f"  {key}: {value}")
    else:
        print(f"{Colors.RED}No se encontraron usuarios{Colors.ENDC}")

if __name__ == "__main__":
    main()