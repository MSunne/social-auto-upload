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
- 这是一个AI的自动化任务平台，结合了OpenClaw和OmniBull，OpenClaw负责AI生成内容，OmniBull负责发布内容到自媒体平台。
- 大型管理平台（OmniDrive_User）负责管理OpenClaw、OmniBull、用户账户，充值，调试等等。
- OmniDrive_Admin管理端包含：OmniBull任务管理、OpenClaw设备管理、OpenClaw任务管理、用户账户管理、充值、调试，计费价格，套餐管理，默认模型，推广人员，分销计费，分销核销，佣金比例，等等。

## 需求：OpenClaw的skills
- OmniDrive_skills: 在OpenClaw中绑定OmniDrive的账户，绑定成功后，可以使用OpenClaw进行图片生成和视频生成，使用OmniDrive中的大模型实现思考，聊天用户不用再去额外接入大模型。
- OmniBull_skills: 在OpenClaw中绑定了OmniDrive的账户并且已经通过OmniBull绑定了自媒体平台的用户。 
  - 可以使用OmniBull进行内容发布，用户可以上传图片，视频，文字，选择自媒体平台。
  - 可以通过自然聊天形式，新建产品知识库（文件夹，产品图，产品说明，prompt）。
  - 可以修改已经准备好但是未执行的定时发布任务，例如修改prompt，修改时间，修改产品介绍和参考图。
  - 可以查看完整的发布任务记录。
  - 可以查看即将发布的任务记录（如果作品未生成，则显示任务未准备好）。
  - 

## 需求：OmniDrive_User
- 图片制作：用户可以上传参考图（多张），选择尺寸，比例，提示词，可选AI优化提示词，提交图片制作，查看进度，查看和下载结果。
- 视频制作：用户可以上传参考图（多张），选择分辨率，时长，提示词，可选AI优化提示词，提交视频制作，查看进度，查看和下载结果。
- 聊天：用户可以选择对应的模型进行聊天，设计灵感和创意。

## 需求：OpenClaw的skills
- 媒体发布技能：OpenClaw可以调用该技能，发送内容到对应的自媒体平台，并返回状态信息给OpenClaw和OmniBull，这是一个重点，因为未来所有的图文生成都是本地OpenClaw完成的。
