## Environment Variables (`.env`)

This project relies on environment variables for configuration, managed through a `.env` file. This keeps sensitive and environment-specific settings separate from your main codebase.

### Setup

1.  **Create a `.env` file:** In the root directory of this project, create a new file named `.env`.
2.  **Add the variables:** Copy the following content into your new `.env` file. **Replace the placeholder values** with your actual, specific configurations.

### Required Variables

| Variable      | Description                                                                                             | Example Value   |
| :------------ | :------------------------------------------------------------------------------------------------------ | :-------------- |
| `COUNTRY_ISO` | The ISO 3166-1 alpha-2 country code (e.g., `US` for United States) used during certificate generation.    | `US`            |
| `PROVINCE`    | The province or state (e.g., `California`) used in your certificate's distinguished name.                | `California`        |
| `CITY`        | The city or locality (e.g., `San Francisco`) used in your certificate's distinguished name.             | `San Francisco`        |
| `ORG_NAME`    | The organization name (e.g., `MyCompany Inc.`) included in your certificate's distinguished name.        | `My Company Inc.`    |
| `LAN_DOMAIN`  | The local area network domain name (e.g., `mywebapp.lan`) used to access the application within your LAN. | `mywebapp.lan`|
| `LAN_IP`      | The local IP address (e.g., `192.168.1.123`) of the machine where the application is hosted, for Nginx configuration. | `192.168.1.123` |

### .env Example

COUNTRY_ISO=US<br>
PROVINCE=California<br>
CITY=San Francisco<br>
ORG_NAME=MyCompany Inc.<br>
LAN_DOMAIN=mywebapp.lan<br>
LAN_IP=192.168.1.123<br>
