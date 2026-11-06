# 科研 SVG 图标库选型：Lucide（线性）

- 日期：2026-11-06
- 状态：当前画布节点图标来源；视觉仍可继续替换
- 代码基线：`6a8737d` 后的本地修改

## 结论

画布不再手绘操作图案，也不使用真实彩色科研插画；当前使用 **Lucide**
outline 线性图标（ISC，24×24，2 px 描边）：

https://github.com/lucide-icons/lucide

选择原因是画布节点需要统一、可缩放的线性视觉语言；真实彩色插画在小尺寸节点里
辨识度低、风格不一致。Lucide 比 Tabler 的 `test-pipe` 更接近移液器，且提供
`pipette`、`microscope` 等更贴切的图形。

## 采用的图标

| 语义 | 本地文件 | Lucide 图标 |
|---|---|---|
| 孔板 / 铺板 | `plate.svg` | `grid-3x3` |
| 移液 / 加药 | `pipette.svg` | `pipette` |
| 孵育 | `incubator.svg` | `clock` |
| 读板 | `reader.svg` | `microscope` |
| 默认操作 | `flask.svg` | `flask-conical` |

图标只负责视觉识别，不改变科学操作类型，也不从自由文本标题推断操作含义。
运行时从 `VisualSpec.symbol` 读取 `plate`、`pipette`、`incubator`、`reader`
或 `flask`；缺失时使用 `flask` 默认值。Definition 为 `lab.well_plate` 时使用孔板图。

## 为什么不是其它库

- **Bioicons**：有实验室器具 CC0 SVG，但多数是彩色/写实插画，放进 76×76 节点
  后风格不一致，且小尺寸下细节丢失。已试装后放弃。
- **Tabler Icons**：线性风格统一，但实验室器具名字较泛（如 `test-pipe`），
  当前改用 Lucide 的具体 `pipette`、`microscope`。
- **Reactome Icon Library**：生物通路图标丰富，许可 CC BY 4.0，但偏细胞、蛋白、
  受体，不是实验室器具线性图标。
- **BioRender**：不是开源/可自由再分发资源，不采用。

## 反例

不要因为图标好看就新增 `OperationModule`、LabwareProfile 或第二套操作类型；
`VisualSpec.symbol` 只是 View 层视觉提示。真正新增、删除或重排科学步骤仍必须走
Protocol / Design 的正式命令。

## 来源

- Lucide repository: https://github.com/lucide-icons/lucide
- Lucide license: https://github.com/lucide-icons/lucide/blob/main/LICENSE
- Lucide icons: https://lucide.dev/icons
- Bioicons (evaluated, not used): https://github.com/duerrsimon/bioicons
- Reactome license: https://reactome.org/license
- Reactome icon library: https://www.reactome.org/icon-lib
