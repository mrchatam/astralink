package app.astralink.client.core

import android.content.Context
import android.util.Log
import app.astralink.client.model.ConnectionProfile
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.util.concurrent.atomic.AtomicBoolean

class AstraLinkProcessManager(private val context: Context) {
    private var process: Process? = null
    private val running = AtomicBoolean(false)

    fun start(profile: ConnectionProfile, configFile: File, resolversFile: File): Boolean {
        stop()
        val binary = File(context.applicationInfo.nativeLibraryDir, "libastralink_client.so")
        if (!binary.exists()) {
            Log.e(TAG, "Native client missing: ${binary.absolutePath}")
            return false
        }
        configFile.writeText(AstraLinkConfigRenderer.renderClientToml(profile))
        if (!resolversFile.exists()) {
            resolversFile.writeText("8.8.8.8\n1.1.1.1\n")
        }
        val cmd = listOf(
            binary.absolutePath,
            "-config", configFile.absolutePath,
            "-resolvers", resolversFile.absolutePath,
            "-nowait",
        )
        process = ProcessBuilder(cmd)
            .redirectErrorStream(true)
            .directory(context.cacheDir)
            .start()
        running.set(true)
        Thread {
            process?.inputStream?.let { stream ->
                BufferedReader(InputStreamReader(stream)).useLines { lines ->
                    lines.forEach { line -> Log.i(TAG, line) }
                }
            }
            running.set(false)
        }.start()
        return true
    }

    fun stop() {
        process?.destroy()
        process = null
        running.set(false)
    }

    fun isRunning(): Boolean = running.get()

    companion object {
        private const val TAG = "AstraLinkProcess"
    }
}
