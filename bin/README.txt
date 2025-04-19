========================================
DBP2P - Base de Datos NoSQL Descentralizada P2P
========================================

Esta carpeta contiene los binarios compilados de DBP2P para diferentes plataformas:

1. dbp2p.exe - Para Windows 64-bit
2. dbp2p_linux - Para Linux 64-bit

Instrucciones de uso:

Windows:
--------
1. Abra una terminal (cmd o PowerShell)
2. Navegue hasta la carpeta donde se encuentra dbp2p.exe
3. Ejecute el programa:
   > .\dbp2p.exe

Linux:
------
1. Abra una terminal
2. Navegue hasta la carpeta donde se encuentra dbp2p_linux
3. Dé permisos de ejecución al archivo:
   $ chmod +x dbp2p_linux
4. Ejecute el programa:
   $ ./dbp2p_linux

Opciones disponibles:
--------------------
- --api=true/false: Habilitar/deshabilitar API REST (por defecto: true)
- --ws=true/false: Habilitar/deshabilitar WebSocket (por defecto: true)
- --api-port=XXXX: Puerto para la API REST (por defecto: 8080)
- --ws-port=XXXX: Puerto para WebSocket (por defecto: 8081)

Ejemplo:
-------
Windows:
> .\dbp2p.exe --api=true --ws=true --api-port=8080 --ws-port=8081

Linux:
$ ./dbp2p_linux --api=true --ws=true --api-port=8080 --ws-port=8081

Para más información, consulte la documentación completa en el archivo README.md
o visite el repositorio en GitHub: https://github.com/aratan/dbp2p

========================================
