@echo off
REM Script para ejecutar DBP2P en modo CLI con puertos personalizados

REM Configurar puertos
set API_PORT=8090
set WS_PORT=8091

REM Ejecutar DBP2P en modo CLI
dbp2p.exe --api-port=%API_PORT% --ws-port=%WS_PORT% cli

pause
