@echo off
REM Script para ejecutar DBP2P en modo CLI sin API ni WebSocket

REM Ejecutar DBP2P en modo CLI
dbp2p.exe --api=false --ws=false cli

pause
