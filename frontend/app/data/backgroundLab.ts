export type BackgroundStudyId = 'blocks' | 'grid' | 'orbits' | 'type' | 'folds' | 'pixels' | 'kinetic' | 'papercut' | 'city' | 'poster' | 'cosmos' | 'garden' | 'kinetic-sculpture' | 'city-future' | 'kinetic-playground' | 'balance-seesaw' | 'balance-orbit' | 'marble-tower' | 'switchyard' | 'business-city' | 'harbor-city'
export interface BackgroundStudy {
  id: BackgroundStudyId
  group?: 'scene' | 'focus' | 'variant'
  comparison?: BackgroundStudyId
  interactive?: boolean
  name: string
  english: string
  description: string
  interaction: string
  paper: string
  ink: string
  accent: string
  reference: { name: string, href: string }
}
export const backgroundStudies: BackgroundStudy[] = [
  { id: 'blocks', name: '构建群岛', english: 'ASSEMBLY / 01', description: '等距块体错峰升起，形成一座持续搭建的微缩城市。', interaction: '移动鼠标，改变观察偏移。', paper: '#f1f0ea', ink: '#25333c', accent: '#8c5b42', reference: { name: 'Three.js · Minecraft 几何', href: 'https://threejs.org/examples/webgl_geometry_minecraft.html' } },
  { id: 'grid', name: '结构透视', english: 'BLUEPRINT / 02', description: '建筑框架从消失点连续展开，镜头沿结构向前推进。', interaction: '移动鼠标，改变透视中心。', paper: '#122235', ink: '#f0f3f5', accent: '#a4c4d6', reference: { name: 'Three.js · Fat Wireframe', href: 'https://threejs.org/examples/webgl_lines_fat_wireframe.html' } },
  { id: 'orbits', name: '轨道图谱', english: 'CONSTELLATION / 03', description: '少量节点沿不同倾角的轨道运行，连线组成有秩序的技术星图。', interaction: '指针靠近节点，局部结构会回应。', paper: '#101822', ink: '#f4f0e9', accent: '#d4b480', reference: { name: 'Three.js · Draw Range 连线', href: 'https://threejs.org/examples/webgl_buffergeometry_drawrange.html' } },
  { id: 'type', name: '字码编舞', english: 'KINETIC TYPE / 04', description: '巨大的代码括号与字行错向穿行，构成有节奏的动态版面。', interaction: '移动鼠标，轻推字行的倾角。', paper: '#f5f2eb', ink: '#292c2b', accent: '#a34e37', reference: { name: 'Codrops · Kinetic Type Page Transition', href: 'https://tympanus.net/Development/KineticTypePageTransition/' } },
  { id: 'folds', name: '折页剧场', english: 'PAPER / 05', description: '相连纸面像百叶一样依次翻转，折面的明暗让空间显现。', interaction: '移动鼠标，改变纸面的开合。', paper: '#eee9e0', ink: '#34302c', accent: '#8c5c48', reference: { name: 'Daniel Velasquez / Codrops · FoldingDOM', href: 'https://tympanus.net/Tutorials/FoldingDOM/' } },
  { id: 'pixels', name: '像素凝聚', english: 'REASSEMBLY / 06', description: '离散方格聚合成巨大的代码括号，再有序松散、重新归位。', interaction: '鼠标经过时，附近像素向外弹开。', paper: '#eaf0f3', ink: '#173e50', accent: '#3a7387', reference: { name: 'Three.js · Interactive Points', href: 'https://threejs.org/examples/webgl_interactive_points.html' } },
  { id: 'kinetic', group: 'scene', name: '动能工坊', english: 'KINETIC ATELIER / 07', description: '一台完整的桌上动能雕塑：齿轮啮合带动偏心连杆与往复活塞，金属构件在同一节律里协作。', interaction: '移动鼠标，轻移雕塑的观察角度。', paper: '#e8e6d9', ink: '#2d4239', accent: '#8d6a3e', reference: { name: 'Ion Drimba Filho · Pendulum', href: 'https://codepen.io/iondrimba/pen/zbOBRO' } },
  { id: 'papercut', group: 'scene', name: '层叠地景', english: 'PAPER TOPOGRAPHY / 08', description: '拱形纸雕框里，山脊、村落与小径一层层向远处退去，飞鸟掠过安静的天空。', interaction: '移动鼠标，观察前后纸层不同幅度的视差。', paper: '#f1ecdf', ink: '#294c40', accent: '#b97d58', reference: { name: 'André Mattos / Codrops · Interactive Landscape', href: 'https://tympanus.net/Development/InteractiveLandscape/' } },
  { id: 'city', group: 'scene', name: '微型城市', english: 'CITY IN MOTION / 09', description: '一座有街道、窗灯、树木与行进车辆的微缩街区，在建筑与车流之间展开日常节奏。', interaction: '移动鼠标，轻推城市的前后景。', paper: '#e9ece6', ink: '#233c3b', accent: '#a66d50', reference: { name: 'Ayman Elbanhawy · Isometric City', href: 'https://codepen.io/aymanelbanhawy/pen/QWBZMmv' } },
  { id: 'poster', group: 'scene', name: '重排编辑室', english: 'EDITORIAL IN MOTION / 10', description: '一张有版心、边注与图形构成的完整海报，在圆形、竖幅与横幅之间连续重排。', interaction: '鼠标轻推纸张，版面按自己的节奏变形。', paper: '#e8e3d5', ink: '#303b30', accent: '#b46240', reference: { name: 'marioecg / Codrops · Kinetic Typography', href: 'https://tympanus.net/Tutorials/codrops-kinetic-typo/' } },
  { id: 'cosmos', group: 'scene', name: '行星观测站', english: 'PLANETARY OBSERVATORY / 11', description: '带体积明暗的行星、前后交错的星环与卫星，构成有远近关系的缓慢天文镜头。', interaction: '移动鼠标，改变观测视差。', paper: '#0c1426', ink: '#eef1fa', accent: '#cfb783', reference: { name: 'Three.js · Earth', href: 'https://threejs.org/examples/webgpu_tsl_earth.html' } },
  { id: 'garden', group: 'scene', name: '生长花园', english: 'LIVING BOTANICAL / 12', description: '枝干、叶片与花朵在不同高度依次展开，前景和远景植物以不同幅度随风呼吸。', interaction: '移动鼠标，像从植物旁轻轻拂过。', paper: '#e6eadc', ink: '#264437', accent: '#ad795f', reference: { name: 'Three.js · Instancing Scatter', href: 'https://threejs.org/examples/webgl_instancing_scatter.html' } },

  { id: 'kinetic-sculpture', group: 'focus', interactive: true, comparison: 'kinetic', name: '平衡之弧', english: 'KINETIC SCULPTURE / 13', description: '更大尺度的分层动能装置：动力单元、前后皮带与快慢转子、陶瓷主轮和偏心曲柄逐级传递，驱动滑台、平衡臂、大悬球与托盘。点击动力轮改变转速，或拖放小球观察重力回摆。', interaction: '点动力轮加速；点击或横向拖动平衡球，再释放。', paper: '#e8e8d9', ink: '#294537', accent: '#9b8050', reference: { name: 'David C. Roy · Wood That Works（动力关系启发）', href: 'https://www.woodthatworks.com/' } },
  { id: 'city-future', group: 'focus', interactive: true, comparison: 'city', name: '疏朗沙盘', english: 'CITY MODEL / 14', description: '沿用原版微型城市的斜俯视角、立体楼体和暖绿材质，减少重复楼数、拓宽街道，留出广场与街区间距。保留桌上建筑沙盘的尺度与接地感，点击局部设施或车辆查看响应。', interaction: '点大型屋顶风机、观景电梯或近景巴士；可重置再玩。', paper: '#e7ede8', ink: '#23433e', accent: '#a17856', reference: { name: 'Ayman Elbanhawy · Isometric City（等距建筑与街道启发）', href: 'https://codepen.io/aymanelbanhawy/pen/QWBZMmv' } },

  { id: 'kinetic-playground', group: 'focus', interactive: true, comparison: 'kinetic-sculpture', name: '连锁游乐台', english: 'KINETIC PLAYGROUND / 15', description: '点铜球放行：回旋轨、杠杆、白球与铜钟依次接力，最后沿升降回流轨回到起点。另一种完整的桌上动能玩法，可随时重置再玩。', interaction: '点铜球启动连锁；也可单独敲铜钟。', paper: '#eee7d8', ink: '#39483b', accent: '#a47b47', reference: { name: 'David C. Roy · Wood That Works（因果运动启发，原创球道构图）', href: 'https://www.woodthatworks.com/' } },


  { id: 'balance-seesaw', group: 'variant', interactive: true, comparison: 'kinetic-sculpture', name: '横衡摆阵', english: 'BALANCE ARRAY / 16', description: '横向支点与天平摆阵：拨动平衡盘，或切换配重，让横梁与悬球形成不同的平衡状态。', interaction: '点配重切换平衡；拨动大摆球。', paper: '#eee9dc', ink: '#304b3e', accent: '#a67c46', reference: { name: 'David C. Roy · Wood That Works（受控运动启发，原创机构）', href: 'https://www.woodthatworks.com/' } },
  { id: 'balance-orbit', group: 'variant', interactive: true, comparison: 'kinetic-sculpture', name: '双摆轨仪', english: 'ORBITAL BALANCE / 17', description: '竖向轨道与双摆雕塑：在环形支架之间观察不同相位，拨动悬球或点击动力部件改变联动。', interaction: '拨动悬球；点击动力部件改变节律。', paper: '#eee9dc', ink: '#304b3e', accent: '#a67c46', reference: { name: 'David C. Roy · Wood That Works（受控运动启发，原创机构）', href: 'https://www.woodthatworks.com/' } },
  { id: 'marble-tower', group: 'variant', interactive: true, comparison: 'kinetic-playground', name: '层落弹珠塔', english: 'MARBLE TOWER / 18', description: '竖向多层落球机构：点击放球，沿层叠接力机构落下；各层运动随球经过而响应。', interaction: '点顶端球放行，观察逐层接力。', paper: '#eee9dc', ink: '#304b3e', accent: '#a67c46', reference: { name: 'David C. Roy · Wood That Works（受控运动启发，原创机构）', href: 'https://www.woodthatworks.com/' } },
  { id: 'switchyard', group: 'variant', interactive: true, comparison: 'kinetic-playground', name: '分岔接力台', english: 'SWITCHYARD / 19', description: '横向分岔轨道：先切换道岔，再放球，选择不同轨迹与跷跷板接力。', interaction: '先点道岔切换路线，再点球放行。', paper: '#eee9dc', ink: '#304b3e', accent: '#a67c46', reference: { name: 'David C. Roy · Wood That Works（受控运动启发，原创机构）', href: 'https://www.woodthatworks.com/' } },

  { id: 'business-city', group: 'variant', interactive: true, comparison: 'city', name: '商务模型城', english: 'BUSINESS MODEL / 20', description: '扩大的多街区商务沙盘：九栋商务楼、公交站、公园、水岸及摩天轮、过山车休闲区分布其间。小飞机穿越整页，飞艇环绕城市巡航，二者点击尾部喷气加速后回巡航，飞机带航空表演式三股红白蓝烟羽，短时开烟后断开扩散消散；直升机起降、蓝白公交沿道路完成 16 秒闭环并回站、游船乘浪回泊、独立东侧广场的四种精细喷泉水型、摩天轮慢转、过山车横向展开，14 秒慢爬、顶端放缓、急降与 S 弯减速回站、行人过街，以及城市日照切换。', interaction: '飞机与飞艇自动巡航，点击尾部喷气加速后恢复，飞机短时拉出三股并排彩烟后断开消散；太阳切换城市日照。', paper: '#e8ede5', ink: '#28483e', accent: '#a57951', reference: { name: 'Ayman Elbanhawy · Isometric City（原创现代模型城构图）', href: 'https://codepen.io/aymanelbanhawy/pen/QWBZMmv' } },
  { id: 'harbor-city', group: 'variant', interactive: true, comparison: 'city', name: '滨水桥港城', english: 'HARBOR MODEL / 21', description: '滨水街区围绕河道和桥梁展开，错落高楼、立体道路车辆、河船与停机坪组成另一种模型轮廓。点击桥梁开合，或让直升机起降。', interaction: '点桥梁开合；点直升机起降。', paper: '#e8ede5', ink: '#28483e', accent: '#a57951', reference: { name: 'Ayman Elbanhawy · Isometric City（原创现代模型城构图）', href: 'https://codepen.io/aymanelbanhawy/pen/QWBZMmv' } },
]
