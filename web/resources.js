window.AsterResources = {
  instruments: [
    {id:"plate-reader", title:"酶标仪 / Plate reader", summary:"吸光度、荧光、发光；记录实际波长、通道和原始读数。", tags:["absorbance","fluorescence"]},
    {id:"qpcr", title:"实时荧光定量 PCR 仪", summary:"96 孔、FAM/HEX 等通道；记录热程序、通道和原始 Cq。", tags:["qPCR","Cq"]},
    {id:"incubator", title:"CO₂ 培养箱", summary:"温度、CO₂、湿度与避光条件；实际值由执行记录填写。", tags:["culture","incubation"]},
    {id:"centrifuge", title:"离心机", summary:"管型、转速/相对离心力、时间与温度。", tags:["separation"]},
    {id:"microscope", title:"倒置/荧光显微镜", summary:"物镜、通道、曝光、视野与图像文件。", tags:["imaging","fluorescence"]},
    {id:"washer", title:"洗板机 / 洗板", summary:"洗液、体积、次数、浸泡与残留控制。", tags:["wash"]},
    {id:"pipette", title:"单通道/多通道移液器", summary:"量程、吸头、校准状态；不自动推断体积。", tags:["liquid-transfer"]},
    {id:"biosafety", title:"生物安全柜", summary:"操作级别与气流状态；属于执行条件，不是实验事实。", tags:["containment"]}
  ],
  tools: [
    {id:"dilution", title:"稀释计算", summary:"C1V1 = C2V2；显示假设和单位。", view:"calculators", mode:"dilution"},
    {id:"reagent-total", title:"试剂总量", summary:"孔位数 × 每孔体积 × (1 + 额外比例)。", view:"calculators", mode:"reagent"},
    {id:"serial-dilution", title:"系列稀释", summary:"逐级转移与稀释倍数；保留每一步输入。", view:"calculators", mode:"serial"},
    {id:"reaction-volume", title:"反应体系", summary:"目标体积、各组分体积与剩余体积。", view:"calculators", mode:"reaction"},
    {id:"plate-layout", title:"孔板布局", summary:"按实际孔位安排组别、样本与对照。", view:"plates"},
  ],
  materials: [
    {id:"media", title:"培养基 / 缓冲液", summary:"配方、批号、体积和有效期由 Protocol/执行记录提供。"},
    {id:"cck8", title:"CCK-8 试剂", summary:"体积、孵育时间、空白与读板波长按方法记录。"},
    {id:"rt-kit", title:"RT / qPCR 试剂盒", summary:"酶、mix、引物/探针、批号与反应体积。"},
    {id:"stain", title:"染色/脱色液", summary:"染料、溶剂、浓度、体积与接触时间。"},
    {id:"antibody", title:"抗体 / 探针", summary:"靶点、宿主/克隆、稀释度、批号。"},
    {id:"consumables", title:"耗材", summary:"板型、管型、膜、玻片与吸头；与实验单位分开。"}
  ]
};
