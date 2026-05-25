package app.astralink.client

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import app.astralink.client.model.ConnectionProfile
import app.astralink.client.proxy.AstraLinkProxyService
import app.astralink.client.vpn.AstraLinkVpnService

class MainActivity : ComponentActivity() {
    private val domain = mutableStateOf("")
    private val key = mutableStateOf("")
    private val mode = mutableStateOf("proxy")

    private val vpnPermission = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            connectVpn()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        intent?.data?.let { uri ->
            if (uri.scheme == "astralink") {
                ProfileLinks.import(uri.toString())?.let { profile ->
                    domain.value = profile.domain
                    key.value = profile.encryptionKey
                }
            }
        }

        setContent {
            MaterialTheme {
                Column(
                    modifier = Modifier.fillMaxSize().padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Text("AstraLink")
                    OutlinedTextField(domain.value, { domain.value = it }, label = { Text("Domain") })
                    OutlinedTextField(key.value, { key.value = it }, label = { Text("Encryption key") })
                    Button({ mode.value = "proxy"; connectProxy() }) { Text("Connect proxy") }
                    Button({
                        mode.value = "vpn"
                        val intent = VpnService.prepare(this@MainActivity)
                        if (intent != null) vpnPermission.launch(intent) else connectVpn()
                    }) { Text("Connect VPN") }
                }
            }
        }
    }

    private fun connectProxy() {
        val profile = buildProfile()
        startForegroundService(
            Intent(this, AstraLinkProxyService::class.java)
                .putExtra(AstraLinkProxyService.EXTRA_PROFILE_JSON, ProfileLinks.export(profile)),
        )
    }

    private fun connectVpn() {
        val profile = buildProfile()
        startForegroundService(
            Intent(this, AstraLinkVpnService::class.java)
                .putExtra(AstraLinkVpnService.EXTRA_PROFILE_JSON, ProfileLinks.export(profile)),
        )
    }

    private fun buildProfile(): ConnectionProfile {
        return ConnectionProfile(
            name = "default",
            domain = domain.value.trim().trimEnd('.'),
            encryptionKey = key.value.trim(),
            encryptionMethod = 2,
            connectionMode = mode.value,
        )
    }
}
