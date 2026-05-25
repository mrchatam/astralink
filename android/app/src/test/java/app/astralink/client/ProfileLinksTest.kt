package app.astralink.client

import app.astralink.client.model.ConnectionProfile
import org.junit.Assert.assertEquals
import org.junit.Test

class ProfileLinksTest {
    @Test
    fun roundTripProfileLink() {
        val profile = ConnectionProfile("home", "t.example.com", "secret", 2, "proxy")
        val link = ProfileLinks.export(profile)
        val imported = ProfileLinks.import(link)!!
        assertEquals(profile.domain, imported.domain)
        assertEquals(profile.encryptionKey, imported.encryptionKey)
    }
}
