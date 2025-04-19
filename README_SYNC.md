# Sincronización de Datos en DBP2P

DBP2P es una base de datos NoSQL descentralizada que permite sincronizar datos entre múltiples nodos de forma automática. Este documento explica cómo funciona la sincronización y cómo utilizarla.

## Cómo Funciona la Sincronización

La sincronización en DBP2P utiliza el protocolo libp2p para comunicarse entre nodos y mantener los datos consistentes. Cuando se realiza una operación CRUD (Crear, Leer, Actualizar, Eliminar) en un nodo, esta operación se propaga automáticamente a todos los demás nodos conectados a la red P2P.

### Características Principales

1. **Sincronización Automática**: Todas las operaciones CRUD se sincronizan automáticamente.
2. **Descubrimiento de Nodos**: Los nodos se descubren automáticamente mediante mDNS (en redes locales) y DHT (en Internet).
3. **Resolución de Conflictos**: En caso de conflictos, se utiliza una estrategia de "último en escribir gana".
4. **Sincronización Completa**: Es posible sincronizar manualmente todos los documentos con el comando `sync`.

## Configuración de la Sincronización

La sincronización está habilitada por defecto. Puedes configurarla en el archivo `config.yaml`:

```yaml
network:
  libp2p:
    listen_addresses:
      - "/ip4/0.0.0.0/tcp/4001"
      - "/ip4/0.0.0.0/udp/4001/quic"
    bootstrap_peers:
      - "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
  mdns:
    enabled: true
    service_name: "dbp2p"
    interval: 10
  dht:
    enabled: true
    mode: "server"
    bootstrap_interval: 300
```

### Parámetros de Configuración

- **listen_addresses**: Direcciones en las que el nodo escucha conexiones entrantes.
- **bootstrap_peers**: Nodos de arranque para conectarse a la red P2P.
- **mdns.enabled**: Habilita o deshabilita el descubrimiento de nodos mediante mDNS (para redes locales).
- **mdns.service_name**: Nombre del servicio mDNS.
- **mdns.interval**: Intervalo de anuncio mDNS en segundos.
- **dht.enabled**: Habilita o deshabilita el DHT (Distributed Hash Table) para descubrimiento de nodos en Internet.
- **dht.mode**: Modo de operación del DHT ("server" o "client").
- **dht.bootstrap_interval**: Intervalo de conexión a nodos de arranque en segundos.

## Uso de la Sincronización

### Sincronización Automática

La sincronización automática está habilitada por defecto. Cuando creas, actualizas o eliminas un documento, la operación se propaga automáticamente a todos los nodos conectados.

### Sincronización Manual

Puedes sincronizar manualmente todos los documentos con el comando `sync` en la interfaz de línea de comandos:

```
> sync
Sincronizando todos los documentos...
Sincronización completada
```

### Verificación de Nodos Conectados

Para verificar los nodos conectados, puedes usar la API REST:

```
GET /api/network/peers
```

Respuesta:
```json
{
  "peers": [
    "12D3KooWRzPQ8NFTXzKAEH3fRmmBZGgbqxYRdKtxVCPnkd8sDMxH",
    "12D3KooWMHVqLUNpqXvQMPvbQhS7uaiCyWsiKXMQNL4RdpgMKSJf"
  ],
  "count": 2
}
```

## Solución de Problemas

### Los datos no se sincronizan

1. **Verificar conectividad P2P**: Asegúrate de que los nodos estén conectados entre sí. Puedes verificar los nodos conectados con la API REST.
2. **Verificar configuración de red**: Asegúrate de que mDNS y/o DHT estén habilitados y configurados correctamente.
3. **Verificar firewalls**: Asegúrate de que los puertos necesarios (por defecto, 4001) estén abiertos en tu firewall.
4. **Sincronización manual**: Intenta sincronizar manualmente con el comando `sync`.

### Conflictos de datos

En caso de conflictos (cuando dos nodos modifican el mismo documento), DBP2P utiliza una estrategia de "último en escribir gana". Esto significa que la última modificación recibida será la que prevalezca.

## Ejemplos de Uso

### Ejemplo 1: Sincronización entre dos nodos en la misma red local

1. Inicia el primer nodo:
   ```
   ./dbp2p.exe
   ```

2. Inicia el segundo nodo en otra máquina:
   ```
   ./dbp2p.exe
   ```

3. Los nodos se descubrirán automáticamente mediante mDNS y comenzarán a sincronizar datos.

### Ejemplo 2: Sincronización entre nodos en diferentes redes

1. Configura los nodos con los mismos nodos de arranque en `config.yaml`:
   ```yaml
   network:
     libp2p:
       bootstrap_peers:
         - "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
   ```

2. Inicia los nodos:
   ```
   ./dbp2p.exe
   ```

3. Los nodos se conectarán a los nodos de arranque y se descubrirán entre sí a través del DHT.

## Limitaciones

- La sincronización requiere que los nodos estén conectados a la red P2P.
- En redes con NAT, es posible que necesites configurar el reenvío de puertos para permitir conexiones entrantes.
- La estrategia de resolución de conflictos "último en escribir gana" puede no ser adecuada para todos los casos de uso.
