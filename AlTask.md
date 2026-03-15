## 文档和key
API文档地址：https://docs.apiyi.com/
OpenAI格式文档地址：https://docs.apiyi.com/api-capabilities/openai-sdk
gemini格式地址：https://docs.apiyi.com/api-capabilities/gemini-native-format
Sora2视频官方文档：https://docs.apiyi.com/api-capabilities/sora-2-video-official
Veo视频官方文档：https://docs.apiyi.com/api-capabilities/veo/overview
Nano Banana图片官方文档：https://docs.apiyi.com/api-capabilities/nano-banana-image
Nano Banana图片编辑官方文档：https://docs.apiyi.com/api-capabilities/nano-banana-image-edit
图像理解和视频理解使用：gemini-3.1-pro-preview
视频生成模型默认：veo-3.1-fast-fl
我的通用key:sk-H6zIVLfJ78QcuRYoA2770b8eC51a45A4940d9bAa2c1eF20a
我的sora专用key：sk-hhBf0fVUzbYtgrrg8413E03b3c414a62A9F8Fe3542B32cFc
图片生成和编辑默认模型：gemini-3-pro-image-preview
DB_URL=mysql://root:xhSL.1379@127.0.0.1:3306/omni_cl
PostgreSQL密码：xhSL.1379
端口：5432
# 安全配置 (JWT密钥)
SECRET_KEY=k9m2v8nq4w7z3y6p1r5t8u2s9f4h7j3k2l5q8w1e3r6t9y4u7i2o5p8a3s6d9f2g
# 登录有效期12小时
ACCESS_TOKEN_EXPIRE_MINUTES=720
S3_BUCKET_NAME=ueditor-sunne
S3_ACCESS_KEY_ID=rEWuWh8OCWqj6KW8xk_IaTGgu8U7g803k21BtM00
S3_SECRET_ACCESS_KEY=KMrfMokRkQSzru5mgTp_jyv6oxC8w8PdKJfNLjjG
S3_ENDPOINT_URL=https://ueditor-sunne.s3.cn-south-1.qiniucs.com
S3_PRIVATE_URL=https://qny.sunne.xyz/
IMAGE_STORE_PATH=plus_ai/img
VIDEO_STORE_PATH=plus_ai/video

# 一个企业级的AI自动化自媒体方案
## 名称说明
- OpenClaw：一个超火的Angent，可以安装skill，可以自动执行任务，自主决策，这个你要联网检索，我本地也已经安装，可供你直接使用。
- OmniDrive：核心云平台，聚合了做视频，做图片，聊天思考，OpenClaw管理，任务管理，自媒体账户管理，历史任务记录等等功能。
- OmniSkill: OpenClaw通过这个技能，获得思考模型并使用默认的思考模型（gemini-3.1-pro-preview），生图，改图，做视频，多模态理解能力，获得用户名下的OpenClaw数量，获得用户设置的自媒体账户信息（增删改查），获得任务信息（增删改查）
- SAU：social-auto-upload的缩写，是一个自动化发布自媒体本地程序，集合了多平台，多账户(预计可以60个自媒体账号)，定时任务，及时发布功能。
- SauSkill: OpenClaw通过这个技能，可以获得SAU中当前的任务信息，账户信息，设备状态信息，产品文件增删改查。
- LocaWeb: OpenClaw和SAU都是在Linux独立主机中工作的，所以除了OmniDrive可以管理SAU外，LocaWeb也可以管理，是位于Linux主机中的本地web管理程序（方便运维调整，避免断网后无法处理设备）。
- OmniBull: 安装了OpenClaw和SAU的Linux主机名称（自然也安装了对应的技能），产品代名词，后续提到的SAU、OpenClaw、OmniBull他们都是在一个Linux主机上，提到时候如果任务有关联，则自动关联到一起。
- OmniDriveAdmin：OmniDrive的后端管理平台，本期不在考虑范围内，优先解决前端的工作保障客户使用。

## 需求：OmniDrive
- 图片制作：用户可以上传参考图（多张），选择尺寸，比例，提示词，可选AI优化提示词，提交图片制作，默认制作1张，查看进度，查看和下载结果。
- 视频制作：用户可以上传参考图（多张），选择分辨率，时长，提示词，可选AI优化提示词，提交视频制作，查看进度，默认制作一个视频，查看和下载结果。
- 聊天：用户可以选择对应的模型进行聊天，设计灵感和创意，默认模型gemini-3.1-pro-preview。
- 充值：用户可以选择多种充值方案，默认4种，从后台获取（支付宝，微信，人工充值）。
- 历史记录：默认获取全部历史（视频、图片、聊天），可以分类选择（视频/图片/聊天/成功/失败/时间/分页）
- OpenClaw管理：
  - 管理自己账户下的OpenClaw设备，增加OpenClaw设备，进入页面，默认获取所有的OpenClaw设备，列表中表头信息：名称，状态，推理模型，心跳时间，产品知识和技能（列中显示：详情/编辑技能），媒体账户（列中显示：详情/增加账户），操作（启用和关闭滑块按钮）
  - 增加OpenClaw，用户点击增加OpenClaw，输入对应的设备编码，将设备划归到自己的账户，并且激活设备，可以使用设备中的SAU和OpenClaw相关功能。
  - 可以修改OpenClaw的默认推理模型，适配用户的业务
  - 产品知识和技能详情页面：进去显示技能列表，增加技能按钮，技能列表，表头为技能名称，产品参考图，技能说明，生成内容，操作。
    - 增加技能：用户上传产品参考资料（限定图和文档），输入技能名称，技能输出要求，选择输出类型（图+文，视+文），选择输出模型（选中模型会显示计费方法和模型说明），确认添加技能，回到技能列表。
    - 修改技能：界面和上传技能一致，修改里面的内容，然后保存回到技能列表。
    - 删除技能：删除的时候，会先检测有哪些SAU中的那些账号在使用，提醒用户删除后，这些账户将无法工作，用户的删除按钮灰化倒计时10S后才可以点击，用户点击取消和删除回到技能列表，删除后，会自动同步技能到SAU，SAU收到这个技能被删除后，响应使用到这个技能的账户会停止这个技能并从本地移除产品知识文件和定时任务。
  - 详情/增加账户：
    - 进入页面，上方显示平台总数，账号总数，账号列表，表头：平台，账号名称，最近认证时间，状态，任务（查看任务和增加任务），操作（验证，删除，重新认证）
    - 增加账户：选择平台（抖音，快手，视频号），填写账户名称，点击添加，OmniDrive将向正在操作的这台OmniBull发送账号添加请求，SAU会自动打开无头浏览器，然后打开对应的平台登录页，将用户的登录二维码传回来，用户扫码后实现登录，如果中途遇到二次验证，OmniBull将二次方式传回OmniDrive，用户点击二次认证方式后，OmniDrive会同步给OmniBull，用户填入二次认证码后，点击认证，认证通过实现登录，失败则将二次认证失败的信息同步回来，用户会重新二次添加账号。
    - 查看任务：查看执行的任务列表，表头：平台，发布内容（列中显示截断后的文本），媒体（显示多图或则多文，点击预览按钮可以放大），状态（成功，失败，等待发布），发布时间（未发布则为空）。
    - 增加任务：当给某个账户增加任务的时候，弹窗出现技能列表，用选择其中的一个技能，选择执行时间，可以选择重复执行。
- OpenClaw任务
  - 查看OpenClaw的任务，聊天做图做视频，包括SAU的自动执行任务，也划归到OpenClaw任务中，支持分类选择查看，分页查看。
- 充值：
  - 支持支付宝，微信充值，客服充值，默认有四个套餐（从后端数据库获取）
- 财务管理：
  - 查看自己的消费充值记录。
## OmniSkill
- 大模型聊天思考能力：OpenClaw登录了OmniDrive账户后，OpenClaw可以使用所有的主流大模型，避免自己去配置大模型那么复杂的操作。
- 视频制作：提供视频制作能力，默认模型制作模型是veo3.1-fast。
- 图片制作：提供图片制作能力，文生图，图生图，修改图，默认模型NanaBanana Pro。
- 模型查询：提供模型查询能力，可以分类查询支持的所有模型。
- 余额查询：可以查询自己的账号积分余额。
- 消费查询：可以查询自己的消费充值记录，导出成表格。
- 任务查询：OpenClaw可以查询自己的定时任务信息（只允许查询本机的定时任务信息，按MAC查询，如果用户要查询所有OpenClaw的任务信息，要登录OmniDrive查询）
## SAU
- social-auto-upload的缩写，就是当前工程的缩写。
- 添加平台账号：可以添加抖音、视频号、快手的自媒体账号，并与OmniDrive同步账号信息，如果OmniDrive没有这个账号的信息，会自动同步。
- 产品知识和技能管理：
  - 默认分页显示显示产品技能列表，与OpenClaw管理中，选中某个产品查看的产品知识技能列表一模一样，这里就是OmniDrive中被选中的那个OpenClaw。
  - 添加产品技能：可以添加文件（图和文档），prompt文件，技能名称，输出要求，与OpenClaw管理中的“增加技能”一模一样，这里添加后OmniDrive中的技能列表也会同步。
  - 删除和修改技能：删除和修改技能后会与OmniDrive同步。
- 账号管理：
  - 这里与OmniDrive中的“OpenClaw管理”下的“详情/增加账户”进去后一模一样
  - 账号的增删改查也会同步到OmniDrive
  - 增加任务与“OpenClaw管理”下的“详情/增加账户”下的“增加任务”一模一样，并且同步到OmniDrive
- 状态信息：
  - 查阅OmniBull的基础信息，是否链接OmniDrive，心跳时间，心跳频率，内存使用状况，CPU状况，硬盘使用状况，本机MAC，本机IP。
## SauSkill
- 查阅任务：获得当前SAU的定时任务信息，包括细节，是那个账号执行都要包含在内，如果SAU的定时任务为空，则查询OmniDrive定时任务结果返回给用户。
- 增加任务：给某个账号增加某个产品技能某个时候发，举例说明：用户对OpenClaw说，帮我给某个账号增加一个下午三点发布什么产品到什么平台的任务，任务要求如下、假设输入了图文和提示词这些，然后OpenClaw会向用户再次确认这些信息，确认无误后添加到任务表，如果当天已经错过执行时间，则提示用户任务要到第二天才会执行。
- 查阅账号信息：默认一本地为准，本地为空，则以云为准。
- 删除或则修改任务：删除任务会与OmniDrive同步，保证OmniDrive及时更新，修改任务后，本地记录的时候，也会将图文同步到OmniDrive。
## 重点补充
- SAU的定时任务信息，OmniDrive与OmniBull都存储的有，但是默认是OmniDrive执行，执行完成后，将结果信息（无论是图文还是视文都是以url+文本的形式）同步给OmniBull，OmniBull发布的时候再去S3下载这些内容，接口返回的url和文本供本地使用。
- 原型图中部分细微逻辑有错误，比如视频制作的右上角左下角都有用户个人中心，请斟酌辨别。
- 好几个工程，我只是指定了每个工程的工程名，技术栈你来确定，通信协议你来确定，务必做到代码规范，业务规范，逻辑合理且稳定。
- 如果你不擅长于写前端，我会使用Claude Opus来写代码，你只需要建立工程目录和指定技术栈，任务规划即可。