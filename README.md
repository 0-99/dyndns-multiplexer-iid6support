# dyndns-multiplexer-iid6support

Self-hosted lightweight DynDNS multiplexer for routers like FritzBox. Forwards update requests to multiple providers, supports IPv4/IPv6, custom client addresses via IPv6 Interface ID (IID) and ISP prefix. Easy config via environment variables and Docker.

## Introduction
Many routers (such as FritzBox) only allow configuration of a single DynDNS endpoint. Typically, only the router's own IPv6 address is sent to the DynDNS provider—there is no client-side way to forward individual IPv6 addresses from clients (e.g., reverse proxies or internal hosts). Some DynDNS providers offer zone management, and there are various container solutions that periodically check the client's IP address via PULL requests.  
This project takes a different approach: it does not use PULL, but acts as a self-hosted webhook, allowing you to update multiple DynDNS providers and domains at once. It also enables direct assignment of custom client IPv6 addresses using the Interface ID (IID, "Interface Identifier") and the ISP-assigned IPv6 prefix.

**Successfully tested with the DynDns Services of:**
- [ddnss.de](https://ddnss.de/) (tested 09/2025)
- [strato.com](https://www.strato.com/) (tested 09/2025)

## Features
- HTTP endpoint `/update` for DynDNS update requests
- Forwards requests to multiple DynDNS providers (configured via environment variable)
- Provider config supports URI templates and placeholders (`<domain>`, `<ipaddr>`, `<ip6addr>`, `<ip6lanprefix>`, `<dualstack>`, `<username>`, `<passwd>`)
- Special IPv6 support: If a [provider configuration](#example-provider-configuration) has an Interface ID (IID), the IPv6 address is constructed from prefix + IID
- Access control via environment variables
- Sensitive data masked in logs
- DynDNS v2 protocol status/severity mapping
- Example integration with FritzBox and similar routers

## Quickstart
1. Create a [docker-compose.yml](examples/docker-compose.yml) on any client in your network,  
  Set environment variables:
   - `USER_PASSWORD`: required
   - `PROVIDERS`: JSON array of provider configs ([see below](#example-provider-configuration))
2. Run the [docker-compose.yml](examples/docker-compose.yml)
3. Change the DynDNS configuration on your router.  
    For the FritzBox for example, navigate to
    `http://fritz.box/ → Internet → Permit Access → DynDNS`  
    - **Update-URL**:  
      ```sh
      # Replace 'ip-to-your-container-host' with the IP address of the client running this container
      http://ip-to-your-container-host:8085/update?username=<username>&passwd=<passwd>&domain=<domain>&ipaddr=<ipaddr>&ip6addr=<ip6addr>&ip6lanprefix=<ip6lanprefix>&dualstack=<dualstack>
      ```
    - **Domain name**: `dyndns.multiplexer.internal` *(defined in [env.](#environment-variables)`USER_DOMAIN_NAME`)*
    - **User name**: `user` *(defined in [env.](#environment-variables)`USER_NAME`)*
    - **Password**: The value defined in the [env.](#environment-variables)`USER_PASSWORD`
4. Check if the IP addresses behind the specified domains (environment variable `PROVIDERS`) have been updated

## How it works
- The `/update` endpoint accepts all relevant query parameters:
  - `username`, `passwd`, `domain` (required)
  - `ipaddr`, `ip6addr` (at least one required)
  - `ip6lanprefix`, `dualstack` (optional)
- Placeholders in the provider URI are replaced at runtime:
  - `<username>`, `<passwd>`, `<domain>`: values from provider config
  - `<ipaddr>`, `<ip6lanprefix>`, `<dualstack>`: values from query parameters
  - `<ip6addr>`: If `iid6` is set in the provider, `<ip6lanprefix>` + `iid6` is used *(You may want to take a look at the [IID requirements](#requirements-for-ipv6-iid-interface-identifier))*; otherwise, the value from `ip6addr`

## Example Provider Configuration
```json
[
  {
    "uri": "https://my.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "example" ,
    "passwd": "anotherExample",
    "domain": "exampledomain.my.domain",
    "iid6": "::cafe:babe:dead:beef"
  },
  {
    "uri": "https://another.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "custom" ,
    "passwd": "pwd",
    "domain": "anotherdomain.other.domain"
  },
  {
    "uri": "https://www.ddnss.de/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "example" ,
    "passwd": "$$anotherExampleWithEscapedDollarSign",
    "domain": "worked.with.ddnss.de",
    "iid6": "::cafe:babe:dead:beef"
  },
  {
    "uri": "https://<username>:<passwd>@dyndns.strato.com/nic/update?hostname=<domain>&myip=<ipaddr>,<ip6addr>",
    "username": "example" ,
    "passwd": "anotherExample",
    "domain": "worked.with.strato.com",
    "iid6": "::cafe:babe:dead:beef"
  }
]
```

If the `username`, `passwd` or anything else in your configuration uses a dollar sign `$`:  
Note that in Docker-Compose you need to escape the dollar sign `$` with a second dollar sign `$$`.

You can also add custom attributes to each provider item. While these aren't evaluated by the application, they can be useful for describing the item (like a "description" attribute).

## Example docker-compose
See [examples/docker-compose.yml](examples/docker-compose.yml) for a full configuration example.

## Environment Variables
- `USER_NAME`: Username for incoming requests (optional, default `user`)
- `USER_PASSWORD`: Password for incoming requests (required)
- `USER_DOMAIN_NAME`: Domain for incoming requests (optional, default `dyndns.multiplexer.internal`)
- `PROVIDERS`: JSON array of [provider configs](#example-provider-configuration)
- `LOG_VERBOSE`: Enables verbose logging (optional, default: false). *Use with caution. Sensitive information may be logged if this is **true**.*

## Development
- All logic is in `main.go`
- Only standard Go tools required (`go run`, `go mod tidy`)

## Requirements for IPv6 IID (Interface Identifier)
To use custom IPv6 addresses for a client, the IID (Interface Identifier) must be **stable** and predictable.

In a ***Debian-based system*** using the ***Network Manager (nmcli)***, this can be achieved as follows

1. Find the name of the relevant connection, e.g. for "eth0"
    ```bash
    nmcli -p c
    ```
    Let's assume the name is `Wired connection 1`

2. Show the **ipv6.addr-gen-mode** configuration for the selected connection. ***This value is used as an example in the following commands.***
    ```bash
    # Replace 'Wired connection 1' with the relevant value from step 1
    nmcli -p c show "Wired connection 1" | grep "ipv6.addr-gen-mode"
    ```

3. If the value is "stable-privacy", then a new IID is generated by the client each time the ISP reassigns the IPv6 prefix.  
    While this is good for privacy, it is unsuitable for access from the Internet.  
    **We use the eui64 method instead.**  
    In the default configuration, the *client's MAC address* is used for IID generation.  
    While this can be retained, I suggest **assigning a separate IID**.  
    For this purpose, for example, the client part of the client's IPv4 address in your own network can be used.  
    Let's assume the client's IPv4-Address is `192.168.24.11/24`, then the client part is the value `11` (as a hex value it is `a`), then the value for the assignment is `::a`. This is the value you can define in the `iid6`-Section for a [provider](#example-provider-configuration)

4. **Make sure that this value is not used by any other client in your network.**  
    If you use Unique Local Addresses (ULA) in your network, for example with the ULA-prefix "fd", than you can use.  
    ```bash
    # Replace '::a' with the relevant value from step 3
    ping "fd::a"
    ```

5. **Set the stable IID on the client**  
> [!WARNING]  
> If **Unique Local Addresses (ULA)** is active in your network, the ULA IPv6 address for this client will also change. If the client uses a DNS resolver such as "pi-hole," the changed ULA IPv6 address must be re-entered in the router.

> [!CAUTION]  
> The actions described here are carried out at your own responsibility. Outcomes may vary depending on your environment, and there is no guarantee regarding safety, reliability, or results.

    ```bash
    # Replace 'Wired connection 1' with the relevant value from step 1
    # Replace '::a' with the relevant value from step 3
    sudo nmcli c mod "Wired connection 1" ipv6.addr-gen-mode eui64
    sudo nmcli c mod "Wired connection 1" ipv6.method auto
    sudo nmcli c mod "Wired connection 1" ipv6.token "::a"
    sudo nmcli c up "Wired connection 1"
    # Check if the configuration is set correctly
    # Wait a while so that the network addresses can be recreated
    sleep 5
    nmcli -p c show "Wired connection 1" | grep "ipv6\|IP6"
    ```

> [!NOTE]  
> If you want to reset these settings later, you can restore them using the following commands
> ```bash
> # Replace 'Wired connection 1' with the relevant value from step 1
> sudo nmcli c mod "Wired connection 1" ipv6.token ''
> sudo nmcli c mod "Wired connection 1" ipv6.addr-gen-mode stable-privacy
> sudo nmcli c mod "Wired connection 1" ipv6.method auto
> sudo nmcli c up "Wired connection 1"
> # Check if the configuration is set correctly
> # Wait a while so that the network addresses can be recreated
> sleep 5
> nmcli -p c show "Wired connection 1" | grep "ipv6\|IP6"
> ```

6. (optional) As mentioned in point 5: If **Unique Local Addresses (ULA)** is active in your network, the ULA IPv6 address for this client will also change. If the client uses a DNS resolver such as "pi.hole," the changed ULA IPv6 address must be re-entered in the router.  
    You can find the ULA IPv6 address with the following command
    ```bash
    # get ULA IPv6 address
    nmcli -p c show "Wired connection 1" | grep "IP6\.ADDRESS" | grep "fd"
    ```
> [!TIP]  
> Restarting the router is recommended so that all clients receive the updated DNS address.
> *(you can do this after **step 8**)*

7. For routers such as the FritzBox, the **IID (Interface Identifier)** assigned to the client must now be overwritten in the router configuration.  
> [!IMPORTANT]  
> On routers like FritzBox, the global IPv6 address assigned to a client is typically constructed from the ISP-assigned prefix and the client's IID. For DynDNS to work with custom client addresses, the IID in the router configuration (usually the last 64 bits) must match the IID of the global client IPv6 address.

* For the FritzBox for example, navigate to
`http://fritz.box/ → Home network → Network → edit the relevant client`

> [!CAUTION]  
> The actions described here are carried out at your own responsibility. Outcomes may vary depending on your environment, and there is no guarantee regarding safety, reliability, or results.

* Overwrite the ***IPv6 Interface-ID*** with the relevant value from **step 3**. For the example from step 3 it is `::0:0:0:a`

8. Check the router's port forwarding configuration to make sure the correct IID (Interface Identifier) is being used. If not, reconfigure port forwarding.
> [!TIP]  
> Restarting the router is recommended

## License
[MIT](LICENSE)

