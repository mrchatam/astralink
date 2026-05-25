package app.astralink.client.model

data class ConnectionProfile(
    val name: String,
    val domain: String,
    val encryptionKey: String,
    val encryptionMethod: Int,
    val connectionMode: String,
)
