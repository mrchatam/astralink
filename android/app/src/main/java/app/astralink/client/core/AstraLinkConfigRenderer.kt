package app.astralink.client.core

import app.astralink.client.model.ConnectionProfile

object AstraLinkConfigRenderer {
    fun renderClientToml(profile: ConnectionProfile, listenPort: Int = 18000): String {
        return buildString {
            appendLine("CONFIG_VERSION = 1")
            appendLine("TRANSPORT = \"multipath_quic_dns\"")
            appendLine("MODE = \"simple\"")
            appendLine()
            appendLine("""DOMAINS = ["${escape(profile.domain)}"]""")
            appendLine("DATA_ENCRYPTION_METHOD = ${profile.encryptionMethod}")
            appendLine("ENCRYPTION_KEY = \"${escape(profile.encryptionKey)}\"")
            appendLine("PROTOCOL_TYPE = \"SOCKS5\"")
            appendLine("LISTEN_IP = \"127.0.0.1\"")
            appendLine("LISTEN_PORT = $listenPort")
            appendLine("MAX_ACTIVE_PATHS = 1")
            appendLine("MAX_STANDBY_PATHS = 1")
            appendLine("FEC_ENABLED = false")
            appendLine("RESOLVER_BALANCING_STRATEGY = 3")
            appendLine("LOG_LEVEL = \"INFO\"")
        }.trimEnd()
    }

    private fun escape(value: String): String = value.replace("\\", "\\\\").replace("\"", "\\\"")
}
