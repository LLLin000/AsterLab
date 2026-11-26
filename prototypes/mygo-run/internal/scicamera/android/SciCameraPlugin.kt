package dev.mygo

import android.Manifest
import android.content.ContentValues
import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import android.os.Build
import android.provider.MediaStore
import android.view.MotionEvent
import androidx.camera.core.Camera
import androidx.camera.core.CameraSelector
import androidx.camera.core.FocusMeteringAction
import androidx.camera.core.ImageCapture
import androidx.camera.core.ImageCaptureException
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.content.ContextCompat
import androidx.lifecycle.LifecycleOwner
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.UUID
import java.util.concurrent.TimeUnit
import kotlin.math.atan2
import kotlin.math.sqrt

class SciCameraPlugin : MyGoPlugin {
    private class Cam(val view: PreviewView, val ctx: Context) : SensorEventListener {
        val capture = ImageCapture.Builder().setCaptureMode(ImageCapture.CAPTURE_MODE_MAXIMIZE_QUALITY).build()
        var camera: Camera? = null
        var bound = false
        var error = "the camera is not running"
        var pitchDeg = 0.0
        var rollDeg = 0.0
        var evBias = 0.0
        var exposureLocked = false
        var focusLocked = false
        var wbLocked = false

        private val sensorMgr = ctx.getSystemService(Context.SENSOR_SERVICE) as? SensorManager
        private val accel = sensorMgr?.getDefaultSensor(Sensor.TYPE_GRAVITY)
            ?: sensorMgr?.getDefaultSensor(Sensor.TYPE_ACCELEROMETER)

        fun startSensors() {
            accel?.let { sensorMgr?.registerListener(this, it, SensorManager.SENSOR_DELAY_UI) }
        }

        fun stopSensors() {
            sensorMgr?.unregisterListener(this)
        }

        override fun onSensorChanged(event: SensorEvent?) {
            val v = event?.values ?: return
            if (v.size < 3) return
            val x = v[0].toDouble()
            val y = v[1].toDouble()
            val z = v[2].toDouble()
            val rad2deg = 180.0 / Math.PI
            pitchDeg = atan2(-y, sqrt(x * x + z * z)) * rad2deg
            rollDeg = atan2(x, sqrt(y * y + z * z)) * rad2deg
        }

        override fun onAccuracyChanged(sensor: Sensor?, accuracy: Int) {}

        fun telemetryJson(): JSONObject {
            val out = JSONObject()
            out.put("running", bound)
            out.put("rawSupported", false)
            out.put("iso", 100.0)
            out.put("minISO", 50.0)
            out.put("maxISO", 3200.0)
            out.put("shutterS", 0.01)
            out.put("apertureF", 1.8)
            out.put("evBias", evBias)
            out.put("wbKelvin", 5200.0)
            out.put("wbTint", 0.0)
            out.put("wbGains", JSONArray().put(1.0).put(1.0).put(1.0))
            out.put("maxWBGain", 4.0)
            out.put("focusLensPos", 0.5)
            out.put("pitchDeg", pitchDeg)
            out.put("rollDeg", rollDeg)
            out.put("yawDeg", 0.0)
            out.put("exposureLocked", exposureLocked)
            out.put("wbLocked", wbLocked)
            out.put("focusLocked", focusLocked)
            out.put("deviceModel", "${Build.MANUFACTURER} ${Build.MODEL}")
            out.put("osVersion", "Android ${Build.VERSION.RELEASE}")
            return out
        }
    }

    private val cams = HashMap<Long, Cam>()

    init {
        PluginHost.registerView("scicamera") { context, id, args ->
            val v = PreviewView(context)
            v.implementationMode = PreviewView.ImplementationMode.COMPATIBLE
            val c = Cam(v, context.applicationContext)
            cams[id] = c
            v.setOnTouchListener { view, event ->
                if (event.action == MotionEvent.ACTION_UP) {
                    view.performClick()
                    val cam = c.camera
                    if (cam != null && view.width > 0 && view.height > 0) {
                        val pt = v.meteringPointFactory.createPoint(event.x, event.y)
                        val flags = FocusMeteringAction.FLAG_AF or (if (!c.exposureLocked) FocusMeteringAction.FLAG_AE else 0)
                        val action = FocusMeteringAction.Builder(pt, flags)
                            .setAutoCancelDuration(3, TimeUnit.SECONDS)
                            .build()
                        cam.cameraControl.startFocusAndMetering(action)
                    }
                }
                true
            }
            c.startSensors()
            startCamera(c, args.optBoolean("front"))
            v
        }
    }

    private fun startCamera(c: Cam, front: Boolean) {
        PluginHost.requestPermissions(arrayOf(Manifest.permission.CAMERA)) { granted ->
            if (!granted) {
                c.error = "the app may not use the camera"
                return@requestPermissions
            }
            val ctx = PluginHost.context
            val future = ProcessCameraProvider.getInstance(ctx)
            future.addListener({
                val owner = PluginHost.activity as? LifecycleOwner
                if (owner == null) {
                    c.error = "no window to show the camera in"
                    return@addListener
                }
                val preview = Preview.Builder().build().also { it.surfaceProvider = c.view.surfaceProvider }
                val which = if (front) CameraSelector.DEFAULT_FRONT_CAMERA else CameraSelector.DEFAULT_BACK_CAMERA
                runCatching {
                    val provider = future.get()
                    provider.unbindAll()
                    c.camera = provider.bindToLifecycle(owner, which, preview, c.capture)
                    c.bound = true
                }.onFailure {
                    c.error = it.toString()
                }
            }, ContextCompat.getMainExecutor(ctx))
        }
    }

    private fun saveToMediaStore(ctx: Context, file: File, labID: String, albumName: String): String {
        return runCatching {
            val values = ContentValues().apply {
                put(MediaStore.Images.Media.DISPLAY_NAME, "$labID.jpg")
                put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg")
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    put(MediaStore.Images.Media.RELATIVE_PATH, "Pictures/${albumName.ifBlank { "AsterLab" }}")
                }
            }
            val resolver = ctx.contentResolver
            val uri = resolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values) ?: return ""
            resolver.openOutputStream(uri)?.use { out -> file.inputStream().use { it.copyTo(out) } }
            uri.toString()
        }.getOrDefault("")
    }

    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        val id = args.optLong("id")
        val c = cams[id]
        if (c == null) {
            reply.failure(PluginFailure("NoView"))
            return
        }
        when (method) {
            "telemetry" -> reply.success(c.telemetryJson())
            "configure" -> {
                val cam = c.camera
                if (cam != null) {
                    if (args.has("evBias")) {
                        val bias = args.optDouble("evBias", 0.0)
                        c.evBias = bias
                        val state = cam.cameraInfo.exposureState
                        val step = state.exposureCompensationStep.toDouble()
                        if (step > 0) {
                            val idx = (bias / step).toInt().coerceIn(state.exposureCompensationRange.lower, state.exposureCompensationRange.upper)
                            cam.cameraControl.setExposureCompensationIndex(idx)
                        }
                    }
                    if (args.optString("focusMode") == "point") {
                        val fx = args.optDouble("focusX", 0.5).toFloat()
                        val fy = args.optDouble("focusY", 0.5).toFloat()
                        val pt = c.view.meteringPointFactory.createPoint(fx * c.view.width, fy * c.view.height)
                        val action = FocusMeteringAction.Builder(pt, FocusMeteringAction.FLAG_AF or FocusMeteringAction.FLAG_AE)
                            .setAutoCancelDuration(3, TimeUnit.SECONDS)
                            .build()
                        cam.cameraControl.startFocusAndMetering(action)
                    }
                }
                if (args.has("exposureLocked")) c.exposureLocked = args.optBoolean("exposureLocked")
                if (args.optString("wbMode") == "lock_current" || args.optString("wbMode") == "kelvin") c.wbLocked = true
                if (args.optString("wbMode") == "auto") c.wbLocked = false
                if (args.optString("focusMode") == "lock_current") c.focusLocked = true
                if (args.optString("focusMode") == "auto") c.focusLocked = false
                reply.success(c.telemetryJson())
            }
            "capture" -> {
                if (!c.bound) {
                    reply.failure(PluginFailure("NoCamera", c.error))
                    return
                }
                val ctx = PluginHost.context
                val labID = args.optString("labPhotoID").ifBlank { UUID.randomUUID().toString() }
                val saveToPhotos = args.optBoolean("saveToPhotos", false)
                val albumName = args.optString("albumName", "AsterLab")
                val f = File(ctx.cacheDir, "$labID.jpg")
                c.capture.takePicture(
                    ImageCapture.OutputFileOptions.Builder(f).build(),
                    ContextCompat.getMainExecutor(ctx),
                    object : ImageCapture.OnImageSavedCallback {
                        override fun onImageSaved(r: ImageCapture.OutputFileResults) {
                            val photosId = if (saveToPhotos) saveToMediaStore(ctx, f, labID, albumName) else ""
                            val out = JSONObject()
                            out.put("previewPath", f.absolutePath)
                            out.put("rawFormat", "processed_jpeg_android")
                            out.put("bitDepth", 8)
                            out.put("asShotNeutral", JSONArray().put(1.0).put(1.0).put(1.0))
                            if (photosId.isNotEmpty()) out.put("photosAssetId", photosId)
                            out.put("telemetry", c.telemetryJson())
                            reply.success(out)
                        }

                        override fun onError(e: ImageCaptureException) {
                            reply.failure(PluginFailure("Failed", e.toString()))
                        }
                    }
                )
            }
            "dispose" -> {
                c.stopSensors()
                cams.remove(id)
                runCatching { ProcessCameraProvider.getInstance(PluginHost.context).get().unbindAll() }
                reply.success(null)
            }
            else -> reply.failure(PluginFailure("NoMethod", method))
        }
    }
}
