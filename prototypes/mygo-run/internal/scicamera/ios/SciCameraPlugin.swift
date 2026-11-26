import AVFoundation
import CoreMotion
import ImageIO
import Photos
import UIKit

private final class SciPreviewView: UIView, AVCapturePhotoCaptureDelegate {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }

    let session = AVCaptureSession()
    let output = AVCapturePhotoOutput()
    let sessionQueue = DispatchQueue(label: "party.asterlab.scicamera.session", qos: .userInitiated)
    let ioQueue = DispatchQueue(label: "party.asterlab.scicamera.io", qos: .utility)

    private var device: AVCaptureDevice?
    private let motionManager = CMMotionManager()
    private let stateLock = NSLock()
    private let deviceModel = UIDevice.current.model
    private let osVersion = UIDevice.current.systemVersion
    private var tapRecognizer: UITapGestureRecognizer?
    private let focusReticle = CAShapeLayer()
    private var reticleToken: Int = 0
    private var pitchDeg: Double = 0
    private var rollDeg: Double = 0
    private var yawDeg: Double = 0

    private var pendingReply: PluginReply?
    private var pendingCaptureRaw: Bool = true
    private var pendingSaveToPhotos: Bool = false
    private var pendingAlbumName: String = "AsterLab"
    private var pendingBaseName: String = ""
    private var pendingPreviewData: Data?
    private var pendingRawData: Data?
    private var pendingTelemetrySnapshot: [String: Any] = [:]

    func start(front: Bool) {
        if let pv = layer as? AVCaptureVideoPreviewLayer {
            pv.session = session
            pv.videoGravity = .resizeAspect
        }
        isUserInteractionEnabled = true
        if tapRecognizer == nil {
            let tap = UITapGestureRecognizer(target: self, action: #selector(handlePreviewTap(_:)))
            addGestureRecognizer(tap)
            tapRecognizer = tap
        }
        if focusReticle.superlayer == nil {
            let box = CGRect(x: 0, y: 0, width: 68, height: 68)
            focusReticle.bounds = box
            focusReticle.path = UIBezierPath(roundedRect: box, cornerRadius: 8).cgPath
            focusReticle.strokeColor = UIColor.systemYellow.cgColor
            focusReticle.fillColor = UIColor.clear.cgColor
            focusReticle.lineWidth = 1.5
            focusReticle.opacity = 0
            layer.addSublayer(focusReticle)
        }
        startMotion()
        AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
            guard let self = self, granted else { return }
            self.sessionQueue.async {
                let pos: AVCaptureDevice.Position = front ? .front : .back
                // Lock to true physical 1x wide camera to prevent ultra-wide macro jumps.
                guard let dev = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: pos),
                      let input = try? AVCaptureDeviceInput(device: dev) else { return }
                self.session.beginConfiguration()
                if self.session.canSetSessionPreset(.photo) {
                    self.session.sessionPreset = .photo
                }
                if self.session.canAddInput(input) {
                    self.session.addInput(input)
                }
                if self.session.canAddOutput(self.output) {
                    self.session.addOutput(self.output)
                    if #available(iOS 14.3, *) {
                        // Force linear Bayer RAW instead of computationally fused Apple ProRAW.
                        if self.output.isAppleProRAWSupported {
                            self.output.isAppleProRAWEnabled = false
                        }
                    }
                }
                self.session.commitConfiguration()
                self.device = dev
                self.session.startRunning()
            }
        }
    }

    func stop() {
        stopMotion()
        sessionQueue.async { [weak self] in
            guard let self = self else { return }
            if self.session.isRunning {
                self.session.stopRunning()
            }
            self.device = nil
        }
    }

    private func startMotion() {
        guard motionManager.isDeviceMotionAvailable else { return }
        motionManager.deviceMotionUpdateInterval = 0.1
        let q = OperationQueue()
        q.qualityOfService = .utility
        motionManager.startDeviceMotionUpdates(to: q) { [weak self] motion, _ in
            guard let self = self, let m = motion else { return }
            let deg = 180.0 / Double.pi
            self.stateLock.lock()
            self.pitchDeg = m.attitude.pitch * deg
            self.rollDeg = m.attitude.roll * deg
            self.yawDeg = m.attitude.yaw * deg
            self.stateLock.unlock()
        }
    }

    private func stopMotion() {
        if motionManager.isDeviceMotionActive {
            motionManager.stopDeviceMotionUpdates()
        }
    }

    @objc private func handlePreviewTap(_ gr: UITapGestureRecognizer) {
        let pt = gr.location(in: self)
        showFocusReticle(at: pt)
        guard let pv = layer as? AVCaptureVideoPreviewLayer else { return }
        let devPt = pv.captureDevicePointConverted(fromLayerPoint: pt)
        sessionQueue.async { [weak self] in
            guard let self = self, let dev = self.device else { return }
            do {
                try dev.lockForConfiguration()
                defer { dev.unlockForConfiguration() }
                if dev.isFocusPointOfInterestSupported && dev.isFocusModeSupported(.autoFocus) {
                    dev.focusPointOfInterest = devPt
                    dev.focusMode = .autoFocus
                }
                // Apple AVCam guard: only shift AE target if exposure is not manually locked/custom.
                if dev.exposureMode != .custom && dev.exposureMode != .locked,
                   dev.isExposurePointOfInterestSupported && dev.isExposureModeSupported(.autoExpose) {
                    dev.exposurePointOfInterest = devPt
                    dev.exposureMode = .autoExpose
                }
            } catch {
                NSLog("scicamera tap-to-focus error: %@", error.localizedDescription)
            }
        }
    }

    private func showFocusReticle(at pt: CGPoint) {
        reticleToken += 1
        let token = reticleToken
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        focusReticle.position = pt
        focusReticle.opacity = 1
        focusReticle.transform = CATransform3DMakeScale(1.25, 1.25, 1)
        CATransaction.commit()

        let scale = CABasicAnimation(keyPath: "transform")
        scale.fromValue = NSValue(caTransform3D: CATransform3DMakeScale(1.25, 1.25, 1))
        scale.toValue = NSValue(caTransform3D: CATransform3DIdentity)
        scale.duration = 0.18
        focusReticle.transform = CATransform3DIdentity
        focusReticle.add(scale, forKey: "reticleScale")

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.9) { [weak self] in
            guard let self = self, self.reticleToken == token else { return }
            self.focusReticle.opacity = 0
        }
    }

    private func rawBayerPixelFormat() -> OSType? {
        for fmt in output.availableRawPhotoPixelFormatTypes {
            if #available(iOS 14.3, *) {
                if AVCapturePhotoOutput.isAppleProRAWPixelFormat(fmt) {
                    continue
                }
            }
            return fmt
        }
        return nil
    }

    private func clampedGains(_ dev: AVCaptureDevice, r: Float, g: Float, b: Float) -> AVCaptureDevice.WhiteBalanceGains {
        let hi = max(1.0, dev.maxWhiteBalanceGain)
        return AVCaptureDevice.WhiteBalanceGains(
            redGain: min(hi, max(1.0, r)),
            greenGain: min(hi, max(1.0, g)),
            blueGain: min(hi, max(1.0, b))
        )
    }

    func buildTelemetry() -> [String: Any] {
        stateLock.lock()
        let p = pitchDeg
        let r = rollDeg
        let y = yawDeg
        stateLock.unlock()

        var info: [String: Any] = [
            "running": session.isRunning,
            "rawSupported": rawBayerPixelFormat() != nil,
            "pitchDeg": p,
            "rollDeg": r,
            "yawDeg": y,
            "deviceModel": deviceModel,
            "osVersion": osVersion
        ]
        guard let dev = device else { return info }
        let g = dev.deviceWhiteBalanceGains
        let temp = dev.temperatureAndTintValues(for: clampedGains(dev, r: g.redGain, g: g.greenGain, b: g.blueGain))
        info["iso"] = Double(dev.iso)
        info["minISO"] = Double(dev.activeFormat.minISO)
        info["maxISO"] = Double(dev.activeFormat.maxISO)
        info["shutterS"] = CMTimeGetSeconds(dev.exposureDuration)
        info["apertureF"] = Double(dev.lensAperture)
        info["evBias"] = Double(dev.exposureTargetBias)
        info["wbKelvin"] = Double(temp.temperature)
        info["wbTint"] = Double(temp.tint)
        info["wbGains"] = [Double(g.redGain), Double(g.greenGain), Double(g.blueGain)]
        info["maxWBGain"] = Double(dev.maxWhiteBalanceGain)
        info["focusLensPos"] = Double(dev.lensPosition)
        info["exposureLocked"] = dev.exposureMode == .locked
        info["wbLocked"] = dev.whiteBalanceMode == .locked
        info["focusLocked"] = dev.focusMode == .locked
        info["focalLengthMM"] = Double(dev.activeFormat.videoFieldOfView)
        return info
    }

    func applyConfigure(_ args: [String: Any], reply: @escaping PluginReply) {
        sessionQueue.async { [weak self] in
            guard let self = self else { return }
            guard let dev = self.device else {
                reply(.success(self.buildTelemetry()))
                return
            }
            do {
                try dev.lockForConfiguration()
                defer { dev.unlockForConfiguration() }

                // 1. Exposure control (with safe hardware clamping)
                let reqExpLocked = args["exposureLocked"] as? Bool
                let reqISO = args["iso"] as? Double ?? 0
                let reqShutter = args["shutterS"] as? Double ?? 0
                if reqISO > 0 || reqShutter > 0 || reqExpLocked == true {
                    let targetISO: Float = {
                        if reqISO > 0 {
                            return min(dev.activeFormat.maxISO, max(dev.activeFormat.minISO, Float(reqISO)))
                        }
                        return min(dev.activeFormat.maxISO, max(dev.activeFormat.minISO, dev.iso))
                    }()
                    let targetDur: CMTime = {
                        if reqShutter > 0 {
                            let desired = CMTimeMakeWithSeconds(reqShutter, preferredTimescale: 1_000_000_000)
                            let minD = dev.activeFormat.minExposureDuration
                            let maxD = dev.activeFormat.maxExposureDuration
                            return CMTimeMinimum(maxD, CMTimeMaximum(minD, desired))
                        }
                        return AVCaptureDevice.currentExposureDuration
                    }()
                    if dev.isExposureModeSupported(.custom) && (reqISO > 0 || reqShutter > 0) {
                        dev.setExposureModeCustom(duration: targetDur, iso: targetISO, completionHandler: nil)
                    } else if dev.isExposureModeSupported(.locked) {
                        dev.exposureMode = .locked
                    }
                } else if reqExpLocked == false && dev.isExposureModeSupported(.continuousAutoExposure) {
                    dev.exposureMode = .continuousAutoExposure
                }

                if let bias = args["evBias"] as? Double {
                    let clampedBias = min(dev.maxExposureTargetBias, max(dev.minExposureTargetBias, Float(bias)))
                    dev.setExposureTargetBias(clampedBias, completionHandler: nil)
                }

                // 2. White Balance control (clamped to [1.0, maxWhiteBalanceGain] to avoid ObjC crash)
                let wbMode = args["wbMode"] as? String ?? ""
                if wbMode == "auto" && dev.isWhiteBalanceModeSupported(.continuousAutoWhiteBalance) {
                    dev.whiteBalanceMode = .continuousAutoWhiteBalance
                } else if wbMode == "lock_current" && dev.isWhiteBalanceModeSupported(.locked) {
                    let cur = dev.deviceWhiteBalanceGains
                    dev.setWhiteBalanceModeLocked(with: self.clampedGains(dev, r: cur.redGain, g: cur.greenGain, b: cur.blueGain), completionHandler: nil)
                } else if wbMode == "kelvin" && dev.isWhiteBalanceModeSupported(.locked) {
                    let kelvin = Float(args["wbKelvin"] as? Double ?? 5200)
                    let tint = Float(args["wbTint"] as? Double ?? 0)
                    let rawGains = dev.deviceWhiteBalanceGains(for: AVCaptureDevice.WhiteBalanceTemperatureAndTintValues(temperature: kelvin, tint: tint))
                    dev.setWhiteBalanceModeLocked(with: self.clampedGains(dev, r: rawGains.redGain, g: rawGains.greenGain, b: rawGains.blueGain), completionHandler: nil)
                } else if wbMode == "gains", let arr = args["wbGains"] as? [Double], arr.count >= 3, dev.isWhiteBalanceModeSupported(.locked) {
                    dev.setWhiteBalanceModeLocked(with: self.clampedGains(dev, r: Float(arr[0]), g: Float(arr[1]), b: Float(arr[2])), completionHandler: nil)
                }

                // 3. Focus control
                let focusMode = args["focusMode"] as? String ?? ""
                if focusMode == "auto" && dev.isFocusModeSupported(.continuousAutoFocus) {
                    dev.focusMode = .continuousAutoFocus
                } else if focusMode == "lock_current" && dev.isFocusModeSupported(.locked) {
                    dev.focusMode = .locked
                } else if focusMode == "point" && dev.isFocusPointOfInterestSupported {
                    let fx = min(1.0, max(0.0, args["focusX"] as? Double ?? 0.5))
                    let fy = min(1.0, max(0.0, args["focusY"] as? Double ?? 0.5))
                    dev.focusPointOfInterest = CGPoint(x: fx, y: fy)
                    if dev.isFocusModeSupported(.autoFocus) {
                        dev.focusMode = .autoFocus
                    }
                } else if focusMode == "lens_pos" && dev.isLockingFocusWithCustomLensPositionSupported {
                    let lp = min(1.0, max(0.0, Float(args["focusLensPos"] as? Double ?? Double(dev.lensPosition))))
                    dev.setFocusModeLocked(lensPosition: lp, completionHandler: nil)
                }

                reply(.success(self.buildTelemetry()))
            } catch {
                reply(.failure(PluginFailure("ConfigFailed", error.localizedDescription)))
            }
        }
    }

    func fireCapture(_ args: [String: Any], reply: @escaping PluginReply) {
        sessionQueue.async { [weak self] in
            guard let self = self else { return }
            guard !self.session.outputs.isEmpty, self.session.isRunning else {
                reply(.failure(PluginFailure("NoCamera", "the camera is not running")))
                return
            }
            guard self.pendingReply == nil else {
                reply(.failure(PluginFailure("Busy", "a capture is already in progress")))
                return
            }
            let wantRaw = args["captureRaw"] as? Bool ?? true
            let saveToPhotos = args["saveToPhotos"] as? Bool ?? false
            let albumName = (args["albumName"] as? String ?? "AsterLab").trimmingCharacters(in: .whitespacesAndNewlines)
            let labID = (args["labPhotoID"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)

            self.pendingReply = reply
            self.pendingCaptureRaw = wantRaw
            self.pendingSaveToPhotos = saveToPhotos
            self.pendingAlbumName = albumName.isEmpty ? "AsterLab" : albumName
            self.pendingBaseName = labID.isEmpty ? UUID().uuidString : labID
            self.pendingPreviewData = nil
            self.pendingRawData = nil
            self.pendingTelemetrySnapshot = self.buildTelemetry()

            let settings: AVCapturePhotoSettings
            if wantRaw, let rawFmt = self.rawBayerPixelFormat() {
                settings = AVCapturePhotoSettings(
                    rawPixelFormatType: rawFmt,
                    processedFormat: [AVVideoCodecKey: AVVideoCodecType.jpeg]
                )
            } else {
                settings = AVCapturePhotoSettings(format: [AVVideoCodecKey: AVVideoCodecType.jpeg])
            }
            self.output.capturePhoto(with: settings, delegate: self)
        }
    }

    func photoOutput(_ output: AVCapturePhotoOutput, didFinishProcessingPhoto photo: AVCapturePhoto, error: Error?) {
        if let error = error {
            NSLog("scicamera photoOutput error: %@", error.localizedDescription)
            return
        }
        guard let data = photo.fileDataRepresentation() else { return }
        let isRaw = photo.isRawPhoto
        sessionQueue.async { [weak self] in
            guard let self = self else { return }
            if isRaw {
                self.pendingRawData = data
            } else {
                self.pendingPreviewData = data
            }
        }
    }

    func photoOutput(_ output: AVCapturePhotoOutput, didFinishCaptureFor resolvedSettings: AVCaptureResolvedPhotoSettings, error: Error?) {
        sessionQueue.async { [weak self] in
            guard let self = self, let reply = self.pendingReply else { return }
            let previewData = self.pendingPreviewData
            let rawData = self.pendingRawData
            let baseName = self.pendingBaseName
            let saveToPhotos = self.pendingSaveToPhotos
            let albumName = self.pendingAlbumName
            let telem = self.pendingTelemetrySnapshot

            self.pendingReply = nil
            self.pendingPreviewData = nil
            self.pendingRawData = nil

            if let error = error {
                reply(.failure(PluginFailure("CaptureFailed", error.localizedDescription)))
                return
            }
            guard let jpgData = previewData ?? rawData else {
                reply(.failure(PluginFailure("Failed", "no photo data captured")))
                return
            }

            self.ioQueue.async {
                let tmpDir = URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
                let jpgURL = tmpDir.appendingPathComponent(baseName + (previewData != nil ? ".jpg" : ".dng"))
                var dngURL: URL? = nil
                do {
                    try jpgData.write(to: jpgURL, options: .atomic)
                    if previewData != nil, let rData = rawData {
                        let ru = tmpDir.appendingPathComponent(baseName + ".dng")
                        try rData.write(to: ru, options: .atomic)
                        dngURL = ru
                    }
                } catch {
                    reply(.failure(PluginFailure("WriteFailed", error.localizedDescription)))
                    return
                }

                var asShotNeutral = [1.0, 1.0, 1.0]
                if let gains = telem["wbGains"] as? [Double], gains.count >= 3, gains[0] > 0, gains[1] > 0, gains[2] > 0 {
                    let invR = 1.0 / gains[0]
                    let invG = 1.0 / gains[1]
                    let invB = 1.0 / gains[2]
                    let norm = invG > 0 ? invG : 1.0
                    asShotNeutral = [invR / norm, 1.0, invB / norm]
                }

                let finish = { (photosAssetId: String) in
                    var res: [String: Any] = [
                        "previewPath": jpgURL.path,
                        "rawFormat": dngURL != nil ? "bayer_cfa_dng" : "processed_jpeg",
                        "bitDepth": dngURL != nil ? 12 : 8,
                        "asShotNeutral": asShotNeutral,
                        "telemetry": telem
                    ]
                    if let ru = dngURL {
                        res["rawPath"] = ru.path
                    }
                    if !photosAssetId.isEmpty {
                        res["photosAssetId"] = photosAssetId
                    }
                    reply(.success(res))
                }

                if !saveToPhotos {
                    finish("")
                    return
                }

                self.saveToSystemPhotos(jpgURL: previewData != nil ? jpgURL : nil, dngURL: dngURL ?? (previewData == nil ? jpgURL : nil), baseName: baseName, albumName: albumName) { assetID in
                    finish(assetID)
                }
            }
        }
    }

    private func saveToSystemPhotos(jpgURL: URL?, dngURL: URL?, baseName: String, albumName: String, completion: @escaping (String) -> Void) {
        let handler: (PHAuthorizationStatus) -> Void = { status in
            let allowed = (status == .authorized || status == .limited)
            guard allowed else {
                completion("")
                return
            }
            var placeholderID = ""
            PHPhotoLibrary.shared().performChanges({
                let req = PHAssetCreationRequest.forAsset()
                if let j = jpgURL {
                    let opt = PHAssetResourceCreationOptions()
                    opt.originalFilename = baseName + ".jpg"
                    req.addResource(with: .photo, fileURL: j, options: opt)
                }
                if let d = dngURL {
                    let opt = PHAssetResourceCreationOptions()
                    opt.originalFilename = baseName + ".dng"
                    req.addResource(with: jpgURL != nil ? .alternatePhoto : .photo, fileURL: d, options: opt)
                }
                if let ph = req.placeholderForCreatedAsset {
                    placeholderID = ph.localIdentifier
                    if status == .authorized && !albumName.isEmpty {
                        let fetchOpt = PHFetchOptions()
                        fetchOpt.predicate = NSPredicate(format: "title = %@", albumName)
                        let existing = PHAssetCollection.fetchAssetCollections(with: .album, subtype: .any, options: fetchOpt)
                        if let album = existing.firstObject {
                            if let albumReq = PHAssetCollectionChangeRequest(for: album) {
                                albumReq.addAssets([ph] as NSArray)
                            }
                        } else {
                            let createAlbum = PHAssetCollectionChangeRequest.creationRequestForAssetCollection(withTitle: albumName)
                            createAlbum.addAssets([ph] as NSArray)
                        }
                    }
                }
            }) { ok, _ in
                completion(ok ? placeholderID : "")
            }
        }

        if #available(iOS 14, *) {
            PHPhotoLibrary.requestAuthorization(for: .readWrite, handler: handler)
        } else {
            PHPhotoLibrary.requestAuthorization(handler)
        }
    }
}

final class SciCameraPlugin: MyGoPlugin {
    private var views: [Int: SciPreviewView] = [:]

    init() {
        PluginHost.registerView("scicamera") { [weak self] id, args in
            let v = SciPreviewView()
            self?.views[id] = v
            v.start(front: args["front"] as? Bool ?? false)
            return v
        }
    }

    func call(_ method: String, _ args: [String: Any], _ reply: @escaping PluginReply) {
        guard let id = args["id"] as? Int, let v = views[id] else {
            return reply(.failure(PluginFailure("NoView")))
        }
        switch method {
        case "telemetry":
            v.sessionQueue.async {
                reply(.success(v.buildTelemetry()))
            }
        case "configure":
            v.applyConfigure(args, reply: reply)
        case "capture":
            v.fireCapture(args, reply: reply)
        case "dispose":
            v.stop()
            views[id] = nil
            reply(.success(nil))
        default:
            reply(.failure(PluginFailure("NoMethod", method)))
        }
    }
}
