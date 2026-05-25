package app.astralink.client

import android.util.Base64
import app.astralink.client.model.ConnectionProfile
import org.json.JSONObject
import java.net.URLDecoder
import java.nio.charset.StandardCharsets

object ProfileLinks {
    private const val scheme = "astralink"
    private const val schema = "astralink.profile"
    private const val version = 1

    fun export(profile: ConnectionProfile): String {
        val root = JSONObject()
            .put("schema", schema)
            .put("version", version)
            .put(
                "profile",
                JSONObject()
                    .put("name", profile.name)
                    .put(
                        "server",
                        JSONObject()
                            .put("domain", profile.domain)
                            .put("encryption_key", profile.encryptionKey)
                            .put("encryption_method", profile.encryptionMethod),
                    )
                    .put("mode", profile.connectionMode),
            )
        return "$scheme://${Base64.encodeToString(root.toString().toByteArray(), Base64.URL_SAFE or Base64.NO_WRAP)}"
    }

    fun import(link: String): ConnectionProfile? {
        val raw = link.trim()
        if (!raw.startsWith("$scheme://")) return null
        val payload = raw.removePrefix("$scheme://")
        val decoded = String(
            Base64.decode(URLDecoder.decode(payload, StandardCharsets.UTF_8.name()), Base64.URL_SAFE),
            StandardCharsets.UTF_8,
        )
        val root = JSONObject(decoded)
        if (root.optString("schema") != schema) return null
        val profile = root.getJSONObject("profile")
        val server = profile.getJSONObject("server")
        return ConnectionProfile(
            name = profile.optString("name", "imported"),
            domain = server.getString("domain"),
            encryptionKey = server.getString("encryption_key"),
            encryptionMethod = server.optInt("encryption_method", 2),
            connectionMode = profile.optString("mode", "proxy"),
        )
    }
}
