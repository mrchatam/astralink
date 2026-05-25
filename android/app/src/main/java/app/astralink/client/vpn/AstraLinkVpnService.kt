package app.astralink.client.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import app.astralink.client.ProfileLinks
import app.astralink.client.core.AstraLinkProcessManager
import app.astralink.client.model.ConnectionProfile
import java.io.File

class AstraLinkVpnService : VpnService() {
    private var tunInterface: ParcelFileDescriptor? = null
    private lateinit var processManager: AstraLinkProcessManager

    override fun onCreate() {
        super.onCreate()
        processManager = AstraLinkProcessManager(this)
        ensureChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val profile = intent?.getStringExtra(EXTRA_PROFILE_JSON)?.let { ProfileLinks.import(it) }
            ?: return START_NOT_STICKY
        val config = File(cacheDir, "astralink_client.toml")
        val resolvers = File(cacheDir, "astralink_resolvers.txt")
        if (!processManager.start(profile, config, resolvers)) {
            stopSelf()
            return START_NOT_STICKY
        }
        tunInterface?.close()
        tunInterface = Builder()
            .addAddress("172.19.0.1", 30)
            .addRoute("0.0.0.0", 0)
            .addDnsServer("172.19.0.2")
            .setMtu(1500)
            .setSession("AstraLink")
            .establish()
        startForeground(NOTIFICATION_ID, buildNotification(profile))
        return START_STICKY
    }

    override fun onDestroy() {
        tunInterface?.close()
        processManager.stop()
        super.onDestroy()
    }

    private fun ensureChannel() {
        val mgr = getSystemService(NotificationManager::class.java)
        mgr.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "AstraLink VPN", NotificationManager.IMPORTANCE_LOW),
        )
    }

    private fun buildNotification(profile: ConnectionProfile): Notification {
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("AstraLink VPN")
            .setContentText(profile.domain)
            .setSmallIcon(android.R.drawable.stat_sys_vpn_ic)
            .build()
    }

    companion object {
        const val EXTRA_PROFILE_JSON = "profile_json"
        private const val CHANNEL_ID = "astralink_vpn"
        private const val NOTIFICATION_ID = 42
    }
}
