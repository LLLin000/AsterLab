# AsterLab 科研标准相机与无损图像数据规范（调研纪要）

> 状态：调研与架构边界备忘（2026-11-26）  
> 参考标杆：**Halide**（触控拍摄交互）、**PlantCV** / **micaToolbox (ImageJ/Fiji)**（科研线性校准开源管线）、**Adobe DNG 1.7 规范**、**Nature / Elsevier 科研图像伦理规范**。

---

## 1. 端侧分工与图像处理边界（手机端 vs 桌面端 vs 专业软件）

核心原则：**手机端只负责“物理高保真采集 + 防废片自检”，绝不做破坏性调色；桌面端不做另一个 Photoshop，只承担“线性批量预处理与无损专业软件桥接”。**

| 端 / 工具 | 应该承担的职责 | 明确不做的事 |
|---|---|---|
| **移动端（相机采集端）** | 1. **物理硬件锁定**：锁死 1x 物理主摄（禁跳超广角微距）、最低原生 ISO、50Hz 抗频闪快门、手动白平衡增益、手动对焦位置；<br>2. **真实光学原始捕获**：同步保存 **原生线性 Bayer RAW（`.dng`，严禁用 Apple ProRAW）** + 轻量预览 `.jpg`；<br>3. **无破坏实时辅助**：取景器 20% 历史叠影（Ghost Overlay）、陀螺仪倾角提示、Peak Focus 峰值对焦绿边、0.1s 过曝/模糊提示；<br>4. **全量元数据伴随归档**：生成 `.asterphoto.json` 与实验槽位绑定。 | ❌ 手机端做 AI 磨皮/自动局部 HDR 提亮<br>❌ 手机端裁剪/覆盖原图<br>❌ 塞满屏幕的专业滑块（戴手套无法操作） |
| **AsterLab 桌面端（工作台预处理层）** | 1. **原始底片库不可变托管**：原始 `.dng` + SHA-256 只读存档（对应期刊审稿“提供原始未裁剪未处理原图”要求）；<br>2. **线性批量预处理（Pre-flight）**：基于 `BlackLevel` / `ColorMatrix` / 灰卡 ROI / Flat-Field 空拍平场图，在线性空间做全图统一的白平衡、平场去暗角、曝光归一化与比例尺压印；<br>3. **标准化开放输出**：一键导出整组 **16-bit 线性 TIFF** 或 **300 DPI 标准 sRGB PNG**，方便直接拖入 Lightroom / ImageJ / Illustrator 排 Figure；<br>4. **全局调整审计日志**：自动生成可供 Methods / 图注引用的全局校准参数记录。 | ❌ 复刻 PS 局部画笔、修复/仿制图章（期刊伦理明确禁止）<br>❌ 复刻 Illustrator 矢量排版引擎<br>❌ 生成脆弱的外挂 `.xmp` 去硬猜 Adobe 私有滑块 |
| **外部专业软件**（Lightroom / Darktable / ImageJ / Illustrator） | 接手 AsterLab 整理好的**真线性 `.dng` 或已线性拉齐的 16-bit TIFF**，完成复杂调色、定量测量（创面/抑菌圈面积）与论文矢量排版拼图。 | — |

---

## 2. 湿实验科研拍照与处理的痛点、通用解法与 AsterLab 能力边界

对照湿实验（水凝胶成胶/降解、创面愈合 d0/d3/d7、96 孔板/培养皿、凝胶电泳条带）调研实况，必须诚实区分**“软件能彻底解决的”**与**“必须靠物理配件/实验台条件解决的”**：

| 实验痛点 | 行业通用成熟解法 | AsterLab 软件能否解决 | 诚实边界（物理前提） |
|---|---|---|---|
| **① 跨天/同批背景忽黄忽蓝、忽明忽暗**（满屏蓝布导致 AWB 反向偏色、Smart HDR 局部拉亮） | 拍真 Bayer RAW；每组在画面角落放 **18% 中性灰卡 / 迷你 ColorChecker**；关闭 AWB/AE 全程锁参；50Hz 防频闪快门 | **可完全解决（P0）**<br>锁物理主摄 + 原生 Bayer DNG + 最低原生 ISO + 50/100/200 快门 + 灰卡吸管定白 + 批处理复用 | **严禁直接对着高饱和彩色背景布吸管定白**；若换天拍摄环境光谱变化，仍需画面内包含一小块**中性灰贴片**供后期还原绝对白点 |
| **② 镜头暗角与台灯光照不均**（中心亮边缘暗、手拿偏了背景梯度错位） | 拍摄 1 张无样品空背景 **Flat-Field（平场图）**，后续全组线性除法校正（`micaToolbox` 标准流程） | **可完全解决（P1）**<br>支持同批次标记 1 张 `is_flat_field` 空背景，桌面端一键去暗角和光照梯度 | 空拍平场图必须在**与样品相同的机位与灯位**下拍 1 张 |
| **③ 湿润水凝胶 / 玻璃瓶表面刺眼白色反光（Glare）** | 相机与光源双侧加 **正交偏振片（Cross-Polarization）**，或使用双侧柔光灯箱 | **软件无法直接消除**<br>只能做高光过曝 255 像素实时红框预警提示重拍 | 镜面反光处光子已饱和丢失信息，**软件抠掉或修掉属于科研修图违规**，只能靠柔光箱或偏振片物理消光 |
| **④ 凝胶电泳条带过曝死白 / 弱带不可见** | 实时直方图监控右端不溢出（No Clipping）+ 多组固定低 ISO 长/短快门对照 | **可辅助解决（P0）**<br>取景器内置高亮溢出斑马纹/直方图预警 | 电泳条带严格遵循线性展示，不允许任何局部提亮掩盖背景噪点 |
| **⑤ 无固定机位时主体大小与角度不齐**（图 1–6 西林瓶放大倍率与倾角漂移） | 俯拍架固定高度；或手持时使用水平仪 + 历史半透明叠影 | **可解决约 80%（P0）**<br>锁定焦距 `focus_lens_position` + 20% 上一张 Ghost 叠影 + 陀螺仪 Pitch/Roll 引导 | 20cm 近距手持左右移动 1cm 仍会产生**玻璃高光与弧形背景过渡线的视差**；要达到像素级完全重合仍需数十元的简易手机俯拍支架 |
| **⑥ 期刊投稿被图像查重工具拦截**（Nature / Elsevier / Proofig 审查） | 保留未改动原始 RAW；仅做**全图线性全局调整**；禁止克隆/污点修复；Methods 声明软件与全局处理 | **可完全解决（P0/P1）**<br>只读托管 `.dng` + 校验 SHA-256 + 全流程禁止局部修图工具并导出审计记录 | 研究者本人需保证不在外部分图排版中拼接未声明的泳道或抹除杂点 |

---

## 3. 明确避坑清单（绝不引入的反模式）

1. **禁止将 Apple ProRAW 视作科研 RAW**：ProRAW 虽以 `.dng` 封装，但底层已执行 Photonic Engine 多帧对齐与局部色调映射，像素间存在非线性空间耦合，且可能在锁曝光下暗中触发包围曝光；iOS 端必须捕获 **原生 Bayer RAW（CFA 线性马赛克数据）**。
2. **禁止从高饱和彩色背景（如蓝色实验布四角）反推白平衡**：红/绿通道在深蓝背景上信噪比极低，以此归一会反向污染玻璃瓶内试剂/水凝胶的真实显色。必须使用中性灰卡或极小白色/灰卡贴片（可在出图裁剪时裁去）。
3. **禁止用外挂 `.xmp` 硬猜 Adobe `Temperature / Tint`**：DNG 内嵌元数据与外挂 `.xmp` 易产生覆盖冲突与 Profile 静默回退到 `Adobe Standard`；应直接修改/写入 DNG 标准标签（如 `AsShotNeutral`）或在原生线性空间直接导出 16-bit TIFF。
4. **必须规避 5 大硬件底层漂移陷阱**：
   - **Dual-ISO 硬件增益切换点**：手动 ISO 默认锁定至传感器最低原生 ISO（如 `device.activeFormat.minISO`），避免跨越双原生增益阈值造成亮度台阶跳变；
   - **50Hz / 60Hz 工频频闪**：中国实验室（50Hz）手动快门优先吸附 `1/50s`、`1/100s`、`1/200s`；
   - **传感器热漂移与黑电平**：长时拍摄机身发热会导致暗电流黑电平漂移，解 RAW 必须读取并减去 DNG `BlackLevel`；
   - **镜间色差**：严禁 `UltraWide` 微距自动接管，全程锁死主摄物理镜头 ID；
   - **色域错配**：iPhone 默认 Display P3 在 Windows Office/网页浏览常发灰，科研导出统一使用带明确 ICC 的标准 **sRGB** 或线性 TIFF。

---

## 4. 跨设备、不同拍摄状态下的完整数据保留契约（5 层元数据）

无论手机来自哪代 iPhone/Android，也无论同一组照片隔天拍摄还是更换了不同台面亮度，只要同时保存 **符合 DNG 1.7 的原生线性 Bayer `.dng` + 伴随文件 `.asterphoto.json`**，在电脑端就能最大程度还原“传感器接受到的物理光子状态”，并提供可控的批量对齐参数：

```jsonc
{
  "schema": "asterphoto/1",
  // 【第 1 层：传感器线性与比色还原核心】（确保跨机型颜色统一到 CIE XYZ）
  "raw": {
    "format": "bayer_cfa_dng",
    "bit_depth": 12,
    "smart_hdr_disabled": true,
    "multi_frame_fusion": false,
    "raw_sha256": "...",
    // 注：BlackLevel / WhiteLevel / LinearizationTable / ColorMatrix1..2 /
    // ForwardMatrix1..2 / CalibrationIlluminant1..2 / GainMap(OpcodeList)
    // 均按 DNG 1.7 标准直接内嵌于 .dng 文件头，跨设备无损解析。
    "as_shot_neutral": [0.4812, 1.0, 0.6145]
  },

  // 【第 2 层：物理曝光与抗频闪参数】（记录精确光子放大倍率与电参数）
  "exposure": {
    "iso": 50,
    "device_min_native_iso": 50,
    "shutter_s": 0.01,             // 1/100s，严格对齐 50Hz 电网周期
    "aperture_f": 1.78,
    "ev_bias": 0.0,
    "flicker_grid_hz": 50,
    "exposure_mode": "locked",     // manual_preset | batch_locked | auto_fallback
    "wb_mode": "locked_gains",
    "wb_kelvin_est": 5200,
    "wb_tint_est": 2.0,
    "wb_rgb_gains": [2.078, 1.0, 1.627]
  },

  // 【第 3 层：光学倍率与手持几何姿态】（支撑无固定机位复现与像素尺寸反演）
  "geometry": {
    "device_model": "iPhone16,2",
    "os_version": "iOS 18.2",
    "physical_camera_id": "builtInWideAngleCamera_back",
    "focal_length_mm": 6.765,
    "focal_length_35mm_eq": 24,
    "focus_mode": "locked",
    "focus_lens_position": 0.42,   // 0.0 (近距微距) .. 1.0 (无穷远)
    "imu_pitch_deg": -28.4,
    "imu_roll_deg": 0.6,
    "imu_yaw_deg": 142.1,
    "scale_mm_per_px": null        // 若本批关联定标尺参考图则自动填充换算比例
  },

  // 【第 4 层：环境基准与校正锚点】（决定桌面端如何“一键统一背景”）
  "calibration": {
    "lighting_preset": "bench_led_5000k",
    "reference_target": "neutral_gray_patch_18pct", // none | neutral_gray_patch_18pct | colorchecker_24
    "reference_roi_norm": [0.02, 0.02, 0.08, 0.08], // [x, y, w, h] 归一化坐标，供自动/交互采样
    "reference_linear_rgb": [0.181, 0.180, 0.182],  // 扣除黑电平后的参考贴片线性均值
    "flat_field_asset_id": "img-flat-20261126-001", // 可选：同批空拍背景图，用于去除暗角和光场不均
    "color_space_preview": "sRGB",
    "highlight_clipped_pct": 0.0,
    "blur_laplacian_score": 412.5
  },

  // 【第 5 层：科研实验语义与期刊合规溯源】
  "experiment": {
    "experiment_id": "EXP-20261126-GEL",
    "step_id": "step-3",
    "batch_id": "gel-inversion-batch-a",
    "slot_label": "High-VEGF / d3",
    "timepoint": "d3",
    "frame_role": "sample",        // sample | wb_reference | flat_field | scale_bar_ref
    "batch_ref_asset_id": "img-ref-20261126-001",
    "captured_at": "2026-11-26T21:18:04+08:00",
    "non_destructive_ops": []      // 仅记录桌面端批准的全局线性操作及参数
  }
}
```

### 为什么这套 5 层结构能支持“电脑端最大程度还原 + 快速批量调整”？
1. **换手机也能还原到同一种颜色**：不靠各厂商黑盒 JPG 发色，而是用 DNG 1.7 `ColorMatrix1/2` + `ForwardMatrix` 从各个传感器的私有 RGB 空间映射到统一的 **CIE XYZ / 线性参考白空间**。
2. **换时段/换桌面光强也能拉平背景**：只要在台面边缘贴一块小灰卡（`reference_target`）或拍过灰卡吸管，桌面端在**线性空间**算一条纯全局增益矢量 $\mathbf{g} = \text{RGB}_{\text{ref}} / \text{RGB}_{\text{frame}}$，一次批处理即可把所有图的背景拉到一致，同时**保持水凝胶/试剂/组织的相对真实颜色比值不被破坏**。
3. **倾斜/暗角梯度能物理剥离**：借助可选的 `flat_field_asset_id`（同机位空拍背景图），通过逐像素除法消除镜头边缘暗角与台灯明暗不均，这是 Lightroom 单纯拉白平衡/曝光滑块做不到的。

---

## 5. 成熟交互范式（借鉴 Halide）与建议落地节奏

1. **取景极简 HUD（常态零遮挡）**：顶部单行药丸状态读数（`1x · Bayer RAW · ISO 50 · 1/100s · 5200K`），底部仅保留快门（绑定音量键）、相册缩略图、以及核心开关 `🔒 锁定本批`。
2. **手势直调 + 触觉刻度**：取景画面任意处上下滑微调曝光补偿（带 `0.1 EV` 轻微震动刻度）；点击顶栏单参数后底部展开单行弧形刻度盘（Arc Dial），调完即收。
3. **湿实验专精视觉辅助**：默认提供 **Peak Focus（绿边峰值对焦，解决透明瓶与反光液面拉风箱）**、**高亮过曝斑马纹**、**20% 上一张 Ghost 叠影** 与 **双十字俯拍水平仪**。
4. **分期路线**：
   - **P0（手机高保真采图底座）**：基于 MyGo `CameraPlugin.swift` / Android `CameraX`，补齐 1x 物理主摄锁定、原生 Bayer DNG 双存、WB/ISO/防频闪快门/对焦批锁定、Ghost 叠影 + 水平仪 + 音量键连拍，输出 `.dng + .jpg + .asterphoto.json`。
   - **P1（桌面线性批处理与专业软件桥接）**：桌面端读取同 `batch_id` 组内的 `.dng` + `.asterphoto.json`，提供“一键灰卡/平场线性归一化”导出 16-bit TIFF / 300 DPI sRGB PNG，并自动记录科研图像合规审计日志。
