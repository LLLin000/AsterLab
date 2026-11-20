# Agent 科研绘图（`figure@1` SVG）尺寸规范与可拼装部件库

> 本文档定义 **AsterLab AI Agent 与科研人员编写 `figure@1.svg`** 的统一视觉标准、物理比例尺与底层渲染边界。

---

## 1. 铁律：SVG 的职责边界

- **只承担空间/结构视觉表达**（培养皿布片几何分布、离心分层位置、Transwell 腔室示意、WB/电泳条带泳道顺序、操作流程箭头等）。
- **严禁将结构化实验数据藏入 SVG 文本**（分组、样本、剂量、实测读数必须严格存于 `group_table@1`、`param_table@1` 与 `data_table@1`）。
- 像**抑菌圈、平板划线、牛津杯法**这类在连续琼脂平皿上的空间排布，一律由 Agent 按处理数用本套件绘制 `figure@1.svg`，**不再伪装成离散 `plate@1` 孔板**。

---

## 2. 画布、安全边距与物理比例尺（`1 mm = 2.4 px`）

- **标准默认画布**：`viewBox="0 0 360 280"`，居中构图。
- **文字安全边距**：图形主体距离画布四边保持至少 **`32 px`** 留白；左右角标 `x >= 12` / `x <= 348`，上下留白 `y >= 24` / `y <= 258`，确保中英文标签与 `annotations` 不被裁切。
- **标准湿实验物理比例尺（`1 mm = 2.4 px`）**：

| 实验实物 | 真实物理尺寸 | SVG 像素尺寸（`1 mm = 2.4 px`） |
|---|---|---|
| **90 mm 琼脂培养皿** | 外径 `90 mm` · 内琼脂直径 `85 mm` | 外圆 `r="108"`（直径 `216 px`）· 内圈 `r="102"` |
| **60 mm 细胞培养皿** | 外径 `60 mm` · 内底 `55 mm` | 外圆 `r="72"` · 内圈 `r="66"` |
| **6 mm 药敏滤纸片** | 直径 `6 mm` | `r="7.2"` |
| **8 mm 牛津杯** | 外径 `8 mm` · 内孔 `5 mm` | 外圆 `r="9.6"` · 内孔 `r="6.0"` |
| **CLSI 抑菌圈最小间距** | 片心距 `≥ 24 mm` · 距皿内壁 `≥ 15 mm` | 片心距 **`≥ 58 px`** · 距内圈 **`≥ 36 px`** |

---

## 3. 抑菌圈 / 平板处理 `N = 2..7` 极坐标排布表（中心 `(180, 140)`，分布圆半径 `R = 62 px`）

当 `90 mm` 皿内布置 `N` 个处理（以皿心 `g transform="translate(180, 140)"` 为原点）：

| 处理数目 $N$ | 推荐布局模式 | 分布圆半径 $R$ | 各点极角 / 相对圆心 `(cx, cy)`（约整数像素） |
|---|---|---|---|
| **2 处理** | 水平/对角对称 | `R = 62` | `(-62, 0)`, `(+62, 0)`（中心距 `124 px = 51.7 mm`） |
| **3 处理** | 正三角 120° | `R = 62` | `(0, -62)`, `(+54, +31)`, `(-54, +31)` |
| **4 处理（四象限）** | 45° 四象限十字 | `R = 62` | `(-44, -44)`, `(+44, -44)`, `(-44, +44)`, `(+44, +44)` |
| **5 处理** | 正五边形 72° | `R = 64` | `(0, -64)`, `(+61, -20)`, `(+38, +52)`, `(-38, +52)`, `(-61, -20)` |
| **5+1（外圈 5 + 中心对照）** | 外环 `R = 66` + 中心 `(0, 0)` | `R = 66` | 中心 `(0, 0)` + 外环 5 点（距中心 `66 px = 27.5 mm > 24 mm` CLSI 红线！） |
| **6+1（外圈 6 + 中心对照）** | 正六边形 60° + 中心 `(0, 0)` | `R = 66` | 中心 `(0, 0)` + `(0, -66)`, `(+57, -33)`, `(+57, +33)`, `(0, +66)`, `(-57, +33)`, `(-57, -33)` |

---

## 4. MyGo / A4 PDF 渲染引擎红线（必读，违者图形会缺元素）

AsterLab UI（`mygo/ui.ParseSVG` + `widget_figure.go`）与 A4 PDF 导出器（`pdf_sheet.go`）按如下规约解析 SVG：

1. **禁止 `<marker>`（如 `marker-end="url(#arrow)"`）**：底层渲染器不支持 `<marker>`，箭头会直接消失。请使用标准 `<polygon>` / `<path>` 实心三角形拼出箭头（见 [arrows-and-scalebar.svg](./svg-parts/arrows-and-scalebar.svg)）。
2. **禁止 `<filter>`（阴影/高斯模糊）与 `mix-blend-mode`**：会被忽略或解析异常；如需柔和光环或抑菌透明圈，用半透明 `stroke` + `stroke-dasharray` 或叠加低透明 `fill-opacity="0.18"` 同心圆。
3. **禁止后代组合 CSS 选择器**（如 `#cap rect.st1`）：只支持内联属性（`fill`、`stroke`、`stroke-width`、`stroke-dasharray`、`fill-opacity`、`stroke-opacity`、`transform="translate(...)"`）。
4. **`<text>` 中文高精度叠画规范**：
   - 支持显式 `x`、`y`、`font-size`、`text-anchor`（`start` / `middle` / `end`）以及祖先 `<g transform="translate(tx, ty)">` 坐标累加；
   - **不要**用 CSS `style="text-anchor:middle"`，请直接写成属性 `text-anchor="middle"`；
   - 不要用 `<text transform="rotate(...)">`（文本旋转在叠加文本层不生效）。

---

## 5. 可直接复用的标准 SVG 部件库（`docs/sdk/svg-parts/`）

| 部件文件 | 适用实验场景 | 说明 |
|---|---|---|
| [petri-dish-90mm.svg](./svg-parts/petri-dish-90mm.svg) | 抑菌圈、涂布菌落、贴片分区 | 标准 `1 mm = 2.4 px` 90 mm 琼脂皿俯视 + 四象限十字参考线 |
| [oxford-cup-and-disc.svg](./svg-parts/oxford-cup-and-disc.svg) | 牛津杯法 / 药敏滤纸片 | `6 mm` 纸片、`8 mm` 双环牛津杯及抑菌圈虚线示意模板 |
| [centrifuge-tube-layers.svg](./svg-parts/centrifuge-tube-layers.svg) | 密度梯度离心 / PBMC / 血清分离 | 15 mL 离心管上清液-白膜层-沉淀分层剖面示意图 |
| [transwell-insert.svg](./svg-parts/transwell-insert.svg) | 细胞迁移 / 侵袭 / 共培养 | 上室、聚碳酸酯滤膜与下室化学趋化因子剖面 |
| [gel-wb-lanes.svg](./svg-parts/gel-wb-lanes.svg) | SDS-PAGE / Western Blot / 琼脂糖电泳 | Marker + 多样本泳道条带顺序示意图 |
| [arrows-and-scalebar.svg](./svg-parts/arrows-and-scalebar.svg) | 多步实验流程图与显微比例尺 | 无 `<marker>` 的直箭头、折线箭头与 `100 um / 10 mm` 比例尺积木 |
