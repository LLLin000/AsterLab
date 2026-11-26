import AVFoundation
import CoreMotion
import ImageIO
import Photos
import UIKit

private final class SciPreviewView: UIView, AVCapturePhotoCaptureDelegate, UIGestureRecognizerDelegate {
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

    private var pitchDeg: Double = 0
    private var rollDeg: Double = 0
    private var yawDeg: Double = 0

    // Interactive native gestures & overlays
    private var tapRecognizer: UITapGestureRecognizer?
    private var longPressRecognizer: UILongPressGestureRecognizer?
    private var panRecognizer: UIPanGestureRecognizer?
    private var pinchRecognizer: UIPinchGestureRecognizer?

    private let focusReticle = CAShapeLayer()
    private let evRailLayer = CAShapeLayer()
    private let evSunLayer = CAShapeLayer()
    private let aeafBannerLabel = UILabel()
    private var reticleToken: Int = 0
    private var reticleVisible: Bool = false
    private var lastReticlePoint: CGPoint = .zero
    private var evDragStartBias: Float = 0
    private var pinchStartZoom: CGFloat = 1.0
    private var aeafLockedByTap: Bool = false

    // Apple Camera 60Hz visual level overlays
    private var levelEnabled: Bool = true
    private var wasLevelAligned: Bool = false
    private let levelImpact = UIImpactFeedbackGenerator(style: .light)
    private let lockNotifGen = UINotificationFeedbackGenerator()
    private let levelLeftLayer = CAShapeLayer()
    private let levelRightLayer = CAShapeLayer()
    private let levelCenterLayer = CAShapeLayer()
    private let levelFixedCrossLayer = CAShapeLayer()
    private let levelMoveCrossLayer = CAShapeLayer()

    private var pendingReply: PluginReply?
    private var pendingCaptureFormat: String = "dng+jpg"
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
        setupNativeGesturesAndLayers()
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
            if let dev = self.device, dev.hasTorch, dev.torchMode != .off {
                try? dev.lockForConfiguration()
                dev.torchMode = .off
                dev.unlockForConfiguration()
            }
            if self.session.isRunning {
                self.session.stopRunning()
            }
            self.device = nil
        }
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        let cx = bounds.midX
        let cy = bounds.midY
        levelLeftLayer.position = CGPoint(x: cx, y: cy)
        levelRightLayer.position = CGPoint(x: cx, y: cy)
        levelCenterLayer.position = CGPoint(x: cx, y: cy)
        levelFixedCrossLayer.position = CGPoint(x: cx, y: cy)
        aeafBannerLabel.frame = CGRect(x: cx - 70, y: 12, width: 140, height: 24)
        updateVisualLevelLayers()
    }

    private func setupNativeGesturesAndLayers() {
        if tapRecognizer == nil {
            let tap = UITapGestureRecognizer(target: self, action: #selector(handlePreviewTap(_:)))
            addGestureRecognizer(tap)
            tapRecognizer = tap
        }
        if longPressRecognizer == nil {
            let lp = UILongPressGestureRecognizer(target: self, action: #selector(handlePreviewLongPress(_:)))
            lp.minimumPressDuration = 0.45
            addGestureRecognizer(lp)
            longPressRecognizer = lp
        }
        if panRecognizer == nil {
            let pan = UIPanGestureRecognizer(target: self, action: #selector(handleEVPan(_:)))
            pan.delegate = self
            addGestureRecognizer(pan)
            panRecognizer = pan
        }
        if pinchRecognizer == nil {
            let pinch = UIPinchGestureRecognizer(target: self, action: #selector(handlePinchZoom(_:)))
            addGestureRecognizer(pinch)
            pinchRecognizer = pinch
        }

        if focusReticle.superlayer == nil {
            let box = CGRect(x: 0, y: 0, width: 72, height: 72)
            let rp = UIBezierPath(rect: box)
            // Inner Apple-style midpoint ticks
            rp.move(to: CGPoint(x: box.midX, y: 0)); rp.addLine(to: CGPoint(x: box.midX, y: 7))
            rp.move(to: CGPoint(x: box.midX, y: box.maxY)); rp.addLine(to: CGPoint(x: box.midX, y: box.maxY - 7))
            rp.move(to: CGPoint(x: 0, y: box.midY)); rp.addLine(to: CGPoint(x: 7, y: box.midY))
            rp.move(to: CGPoint(x: box.maxX, y: box.midY)); rp.addLine(to: CGPoint(x: box.maxX - 7, y: box.midY))

            focusReticle.bounds = box
            focusReticle.path = rp.cgPath
            focusReticle.strokeColor = UIColor.systemYellow.cgColor
            focusReticle.fillColor = UIColor.clear.cgColor
            focusReticle.lineWidth = 1.5
            focusReticle.opacity = 0
            focusReticle.zPosition = 10
            layer.addSublayer(focusReticle)

            let rail = UIBezierPath()
            rail.move(to: CGPoint(x: 0, y: -42))
            rail.addLine(to: CGPoint(x: 0, y: 42))
            evRailLayer.bounds = CGRect(x: -2, y: -42, width: 4, height: 84)
            evRailLayer.path = rail.cgPath
            evRailLayer.strokeColor = UIColor.systemYellow.withAlphaComponent(0.7).cgColor
            evRailLayer.lineWidth = 1.2
            evRailLayer.opacity = 0
            evRailLayer.zPosition = 10
            layer.addSublayer(evRailLayer)

            let sunPath = UIBezierPath(ovalIn: CGRect(x: -5, y: -5, width: 10, height: 10))
            evSunLayer.bounds = CGRect(x: -5, y: -5, width: 10, height: 10)
            evSunLayer.path = sunPath.cgPath
            evSunLayer.fillColor = UIColor.systemYellow.cgColor
            evSunLayer.strokeColor = UIColor.black.withAlphaComponent(0.4).cgColor
            evSunLayer.lineWidth = 1.0
            evSunLayer.opacity = 0
            evSunLayer.zPosition = 11
            layer.addSublayer(evSunLayer)
        }

        if aeafBannerLabel.superview == nil {
            aeafBannerLabel.text = "自动曝光/自动对焦锁定"
            aeafBannerLabel.font = UIFont.systemFont(ofSize: 11, weight: .semibold)
            aeafBannerLabel.textColor = .black
            aeafBannerLabel.backgroundColor = UIColor.systemYellow.withAlphaComponent(0.95)
            aeafBannerLabel.textAlignment = .center
            aeafBannerLabel.layer.cornerRadius = 6
            aeafBannerLabel.layer.masksToBounds = true
            aeafBannerLabel.layer.zPosition = 12
            aeafBannerLabel.isHidden = true
            addSubview(aeafBannerLabel)
        }

        if levelLeftLayer.superlayer == nil {
            let dashL = UIBezierPath()
            dashL.move(to: CGPoint(x: -66, y: 0)); dashL.addLine(to: CGPoint(x: -44, y: 0))
            levelLeftLayer.path = dashL.cgPath
            levelLeftLayer.lineWidth = 1.5
            levelLeftLayer.lineCap = .round
            levelLeftLayer.zPosition = 8
            layer.addSublayer(levelLeftLayer)

            let dashR = UIBezierPath()
            dashR.move(to: CGPoint(x: 44, y: 0)); dashR.addLine(to: CGPoint(x: 66, y: 0))
            levelRightLayer.path = dashR.cgPath
            levelRightLayer.lineWidth = 1.5
            levelRightLayer.lineCap = .round
            levelRightLayer.zPosition = 8
            layer.addSublayer(levelRightLayer)

            let centerBar = UIBezierPath()
            centerBar.move(to: CGPoint(x: -28, y: 0)); centerBar.addLine(to: CGPoint(x: 28, y: 0))
            levelCenterLayer.path = centerBar.cgPath
            levelCenterLayer.lineWidth = 1.8
            levelCenterLayer.lineCap = .round
            levelCenterLayer.zPosition = 8
            layer.addSublayer(levelCenterLayer)

            let cross = UIBezierPath()
            cross.move(to: CGPoint(x: -13, y: 0)); cross.addLine(to: CGPoint(x: 13, y: 0))
            cross.move(to: CGPoint(x: 0, y: -13)); cross.addLine(to: CGPoint(x: 0, y: 13))

            levelFixedCrossLayer.path = cross.cgPath
            levelFixedCrossLayer.strokeColor = UIColor.white.withAlphaComponent(0.55).cgColor
            levelFixedCrossLayer.lineWidth = 1.5
            levelFixedCrossLayer.lineCap = .round
            levelFixedCrossLayer.zPosition = 8
            layer.addSublayer(levelFixedCrossLayer)

            levelMoveCrossLayer.path = cross.cgPath
            levelMoveCrossLayer.strokeColor = UIColor.systemYellow.cgColor
            levelMoveCrossLayer.lineWidth = 1.6
            levelMoveCrossLayer.lineCap = .round
            levelMoveCrossLayer.zPosition = 9
            layer.addSublayer(levelMoveCrossLayer)

            levelImpact.prepare()
        }
    }

    private func startMotion() {
        guard motionManager.isDeviceMotionAvailable else { return }
        motionManager.deviceMotionUpdateInterval = 1.0 / 60.0
        let q = OperationQueue()
        q.qualityOfService = .userInteractive
        motionManager.startDeviceMotionUpdates(to: q) { [weak self] motion, _ in
            guard let self = self, let m = motion else { return }
            let deg = 180.0 / Double.pi
            self.stateLock.lock()
            self.pitchDeg = m.attitude.pitch * deg
            self.rollDeg = m.attitude.roll * deg
            self.yawDeg = m.attitude.yaw * deg
            self.stateLock.unlock()
            DispatchQueue.main.async { [weak self] in
                self?.updateVisualLevelLayers()
            }
        }
    }

    private func stopMotion() {
        if motionManager.isDeviceMotionActive {
            motionManager.stopDeviceMotionUpdates()
        }
    }

    private func updateVisualLevelLayers() {
        let hideAll = !levelEnabled
        if hideAll {
            CATransaction.begin()
            CATransaction.setDisableActions(true)
            levelLeftLayer.isHidden = true
            levelRightLayer.isHidden = true
            levelCenterLayer.isHidden = true
            levelFixedCrossLayer.isHidden = true
            levelMoveCrossLayer.isHidden = true
            CATransaction.commit()
            return
        }

        stateLock.lock()
        let p = pitchDeg
        let r = rollDeg
        stateLock.unlock()

        let overhead = abs(p) >= 58.0
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        if overhead {
            levelLeftLayer.isHidden = true
            levelRightLayer.isHidden = true
            levelCenterLayer.isHidden = true
            levelFixedCrossLayer.isHidden = false
            levelMoveCrossLayer.isHidden = false

            let pitchErr = (p >= 0 ? 90.0 - p : -90.0 - p)
            let dx = max(-38.0, min(38.0, r * 2.2))
            let dy = max(-38.0, min(38.0, pitchErr * 2.2))
            let radialErr = sqrt(r * r + pitchErr * pitchErr)
            let aligned = radialErr < 1.5
            if aligned {
                levelMoveCrossLayer.position = CGPoint(x: bounds.midX, y: bounds.midY)
                levelFixedCrossLayer.strokeColor = UIColor.systemYellow.cgColor
                levelMoveCrossLayer.strokeColor = UIColor.systemYellow.cgColor
            } else {
                levelMoveCrossLayer.position = CGPoint(x: bounds.midX + CGFloat(dx), y: bounds.midY + CGFloat(dy))
                levelFixedCrossLayer.strokeColor = UIColor.white.withAlphaComponent(0.55).cgColor
                levelMoveCrossLayer.strokeColor = UIColor.systemYellow.withAlphaComponent(0.95).cgColor
            }
            if aligned && !wasLevelAligned {
                levelImpact.impactOccurred()
            }
            wasLevelAligned = aligned
        } else {
            levelFixedCrossLayer.isHidden = true
            levelMoveCrossLayer.isHidden = true
            levelLeftLayer.isHidden = false
            levelRightLayer.isHidden = false
            levelCenterLayer.isHidden = false

            let aligned = abs(r) <= 1.2
            let angleRad = aligned ? 0.0 : (-r * Double.pi / 180.0)
            let col: CGColor = aligned ? UIColor.systemYellow.cgColor : UIColor.white.withAlphaComponent(0.75).cgColor
            levelLeftLayer.strokeColor = col
            levelRightLayer.strokeColor = col
            levelCenterLayer.strokeColor = aligned ? UIColor.systemYellow.cgColor : UIColor.white.cgColor
            levelCenterLayer.transform = CATransform3DMakeRotation(CGFloat(angleRad), 0, 0, 1)
            if aligned && !wasLevelAligned {
                levelImpact.impactOccurred()
            }
            wasLevelAligned = aligned
        }
        CATransaction.commit()
    }

    func gestureRecognizerShouldBegin(_ gestureRecognizer: UIGestureRecognizer) -> Bool {
        if gestureRecognizer === panRecognizer {
            guard reticleVisible else { return false }
            let vel = panRecognizer?.velocity(in: self) ?? .zero
            return abs(vel.y) > abs(vel.x)
        }
        return true
    }

    @objc private func handlePreviewTap(_ gr: UITapGestureRecognizer) {
        let pt = gr.location(in: self)
        aeafLockedByTap = false
        aeafBannerLabel.isHidden = true
        guard let pv = layer as? AVCaptureVideoPreviewLayer else { return }
        let devPt = pv.captureDevicePointConverted(fromLayerPoint: pt)

        sessionQueue.async { [weak self] in
            guard let self = self, let dev = self.device else { return }
            let customExp = (dev.exposureMode == .custom)
            var curBias: Float = 0
            do {
                try dev.lockForConfiguration()
                defer { dev.unlockForConfiguration() }
                if dev.isFocusPointOfInterestSupported && dev.isFocusModeSupported(.autoFocus) {
                    dev.focusPointOfInterest = devPt
                    dev.focusMode = .autoFocus
                }
                if !customExp,
                   dev.isExposurePointOfInterestSupported && dev.isExposureModeSupported(.autoExpose) {
                    dev.exposurePointOfInterest = devPt
                    dev.exposureMode = .autoExpose
                }
                curBias = dev.exposureTargetBias
            } catch {
                NSLog("scicamera tap-to-focus error: %@", error.localizedDescription)
            }
            DispatchQueue.main.async {
                self.showFocusReticle(at: pt, showEV: !customExp, evBias: curBias, dev: dev, pulseTwice: false)
            }
        }
    }

    @objc private func handlePreviewLongPress(_ gr: UILongPressGestureRecognizer) {
        guard gr.state == .began else { return }
        let pt = gr.location(in: self)
        guard let pv = layer as? AVCaptureVideoPreviewLayer else { return }
        let devPt = pv.captureDevicePointConverted(fromLayerPoint: pt)

        sessionQueue.async { [weak self] in
            guard let self = self, let dev = self.device else { return }
            do {
                try dev.lockForConfiguration()
                defer { dev.unlockForConfiguration() }
                if dev.isFocusPointOfInterestSupported && dev.isFocusModeSupported(.autoFocus) {
                    dev.focusPointOfInterest = devPt
                    dev.focusMode = .locked
                } else if dev.isFocusModeSupported(.locked) {
                    dev.focusMode = .locked
                }
                if dev.exposureMode != .custom && dev.isExposureModeSupported(.locked) {
                    if dev.isExposurePointOfInterestSupported {
                        dev.exposurePointOfInterest = devPt
                    }
                    dev.exposureMode = .locked
                }
            } catch {
                NSLog("scicamera long-press lock error: %@", error.localizedDescription)
            }
            DispatchQueue.main.async {
                self.aeafLockedByTap = true
                self.aeafBannerLabel.isHidden = false
                self.lockNotifGen.notificationOccurred(.success)
                self.showFocusReticle(at: pt, showEV: dev.exposureMode != .custom, evBias: dev.exposureTargetBias, dev: dev, pulseTwice: true)
            }
        }
    }

    @objc private func handleEVPan(_ gr: UIPanGestureRecognizer) {
        guard let dev = device, dev.exposureMode != .custom else { return }
        if gr.state == .began {
            evDragStartBias = dev.exposureTargetBias
            reticleToken += 1
        }
        let dy = -gr.translation(in: self).y // drag up -> brighter
        let delta = Float(dy / 110.0)
        let lo = max(-2.0, dev.minExposureTargetBias)
        let hi = min(2.0, dev.maxExposureTargetBias)
        let nextBias = min(hi, max(lo, evDragStartBias + delta))
        sessionQueue.async {
            try? dev.lockForConfiguration()
            dev.setExposureTargetBias(nextBias, completionHandler: nil)
            dev.unlockForConfiguration()
        }
        positionEVIndicator(bias: nextBias, lo: lo, hi: hi)
        if gr.state == .ended || gr.state == .cancelled {
            scheduleReticleFade()
        }
    }

    @objc private func handlePinchZoom(_ gr: UIPinchGestureRecognizer) {
        guard let dev = device else { return }
        if gr.state == .began {
            pinchStartZoom = dev.videoZoomFactor
        }
        let maxZ = min(dev.activeFormat.videoMaxZoomFactor, 6.0)
        let desired = min(maxZ, max(1.0, pinchStartZoom * gr.scale))
        sessionQueue.async {
            try? dev.lockForConfiguration()
            dev.videoZoomFactor = desired
            dev.unlockForConfiguration()
        }
    }

    private func positionEVIndicator(bias: Float, lo: Float, hi: Float) {
        let sideOffset: CGFloat = (lastReticlePoint.x + 54 > bounds.width - 12) ? -50 : 50
        let railX = lastReticlePoint.x + sideOffset
        let span = max(0.01, hi - lo)
        let norm = CGFloat((bias - lo) / span) // 0=bottom, 1=top
        let sunY = lastReticlePoint.y + 42 - norm * 84
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        evRailLayer.position = CGPoint(x: railX, y: lastReticlePoint.y)
        evSunLayer.position = CGPoint(x: railX, y: sunY)
        CATransaction.commit()
    }

    private func showFocusReticle(at pt: CGPoint, showEV: Bool, evBias: Float, dev: AVCaptureDevice, pulseTwice: Bool) {
        lastReticlePoint = pt
        reticleVisible = true
        reticleToken += 1

        CATransaction.begin()
        CATransaction.setDisableActions(true)
        focusReticle.position = pt
        focusReticle.opacity = 1
        focusReticle.transform = CATransform3DMakeScale(1.25, 1.25, 1)
        if showEV {
            let lo = max(-2.0, dev.minExposureTargetBias)
            let hi = min(2.0, dev.maxExposureTargetBias)
            evRailLayer.opacity = 1
            evSunLayer.opacity = 1
            positionEVIndicator(bias: evBias, lo: lo, hi: hi)
        } else {
            evRailLayer.opacity = 0
            evSunLayer.opacity = 0
        }
        CATransaction.commit()

        let scale = CABasicAnimation(keyPath: "transform")
        scale.fromValue = NSValue(caTransform3D: CATransform3DMakeScale(1.28, 1.28, 1))
        scale.toValue = NSValue(caTransform3D: CATransform3DIdentity)
        scale.duration = 0.18
        if pulseTwice {
            scale.repeatCount = 2
        }
        focusReticle.transform = CATransform3DIdentity
        focusReticle.add(scale, forKey: "reticleScale")

        scheduleReticleFade()
    }

    private func scheduleReticleFade() {
        let token = reticleToken
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.4) { [weak self] in
            guard let self = self, self.reticleToken == token else { return }
            self.reticleVisible = false
            self.focusReticle.opacity = 0
            self.evRailLayer.opacity = 0
            self.evSunLayer.opacity = 0
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
            "aeafLockedByTap": aeafLockedByTap,
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
        info["exposureLocked"] = dev.exposureMode == .locked || dev.exposureMode == .custom
        info["wbLocked"] = dev.whiteBalanceMode == .locked
        info["focusLocked"] = dev.focusMode == .locked
        info["zoomFactor"] = Double(dev.videoZoomFactor)
        info["minZoom"] = 1.0
        info["maxZoom"] = Double(min(dev.activeFormat.videoMaxZoomFactor, 6.0))
        info["torchAvailable"] = dev.hasTorch
        info["torchOn"] = dev.hasTorch && dev.torchMode == .on
        info["focalLengthMM"] = Double(dev.activeFormat.videoFieldOfView)
        return info
    }

    func applyConfigure(_ args: [String: Any], reply: @escaping PluginReply) {
        if let lvl = args["levelEnabled"] as? Bool {
            DispatchQueue.main.async { [weak self] in
                self?.levelEnabled = lvl
                self?.updateVisualLevelLayers()
            }
        }
        sessionQueue.async { [weak self] in
            guard let self = self else { return }
            guard let dev = self.device else {
                reply(.success(self.buildTelemetry()))
                return
            }
            do {
                try dev.lockForConfiguration()
                defer { dev.unlockForConfiguration() }

                // 1. Exposure control
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
                    DispatchQueue.main.async {
                        self.aeafLockedByTap = false
                        self.aeafBannerLabel.isHidden = true
                    }
                }

                if let bias = args["evBias"] as? Double {
                    let clampedBias = min(dev.maxExposureTargetBias, max(dev.minExposureTargetBias, Float(bias)))
                    dev.setExposureTargetBias(clampedBias, completionHandler: nil)
                }

                // 2. White Balance control
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

                // 4. Zoom factor (1.0 .. min(maxZoom, 6.0))
                if let reqZoom = args["zoomFactor"] as? Double, reqZoom > 0 {
                    let maxZ = min(dev.activeFormat.videoMaxZoomFactor, 6.0)
                    dev.videoZoomFactor = min(maxZ, max(1.0, CGFloat(reqZoom)))
                }

                // 5. Constant Lab Torch illumination
                if let wantTorch = args["torchEnabled"] as? Bool, dev.hasTorch {
                    if wantTorch && dev.isTorchModeSupported(.on) {
                        try dev.setTorchModeOn(level: 0.45)
                    } else if !wantTorch && dev.isTorchModeSupported(.off) {
                        dev.torchMode = .off
                    }
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
            let legacyRaw = args["captureRaw"] as? Bool ?? true
            var fmt = (args["captureFormat"] as? String ?? "").lowercased()
            if fmt != "dng+jpg" && fmt != "dng" && fmt != "jpg" {
                fmt = legacyRaw ? "dng+jpg" : "jpg"
            }
            let saveToPhotos = args["saveToPhotos"] as? Bool ?? false
            let albumName = (args["albumName"] as? String ?? "AsterLab").trimmingCharacters(in: .whitespacesAndNewlines)
            let labID = (args["labPhotoID"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)

            self.pendingReply = reply
            self.pendingCaptureFormat = fmt
            self.pendingSaveToPhotos = saveToPhotos
            self.pendingAlbumName = albumName.isEmpty ? "AsterLab" : albumName
            self.pendingBaseName = labID.isEmpty ? UUID().uuidString : labID
            self.pendingPreviewData = nil
            self.pendingRawData = nil
            self.pendingTelemetrySnapshot = self.buildTelemetry()

            let wantRaw = (fmt != "jpg")
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
            let fmt = self.pendingCaptureFormat
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

                let saveJpgURL: URL? = (fmt == "dng" && dngURL != nil) ? nil : (previewData != nil ? jpgURL : nil)
                let saveDngURL: URL? = (fmt == "jpg") ? nil : (dngURL ?? (previewData == nil ? jpgURL : nil))
                self.saveToSystemPhotos(jpgURL: saveJpgURL, dngURL: saveDngURL, baseName: baseName, albumName: albumName) { assetID in
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
