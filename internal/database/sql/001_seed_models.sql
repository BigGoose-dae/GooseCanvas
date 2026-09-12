INSERT INTO model_definitions
  (model_key, name, task_type, provider, protocol, description, inputs, defaults, options, enabled, builtin, created_at, updated_at)
SELECT
  'doubao-seed-2-1-pro-260628', 'Doubao Seed 2.1 Pro', 'text', 'volcengine', 'ark-chat-v3',
  '文本生成与提示词扩写', '["text"]', '{"temperature":0.7,"maxTokens":4096,"thinking":"disabled"}',
  '{"temperature":[0.2,0.7,1],"maxTokens":[1024,2048,4096,8192],"thinking":["disabled","auto","enabled"]}',
  1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM model_definitions WHERE model_key = 'doubao-seed-2-1-pro-260628'
);

INSERT INTO model_definitions
  (model_key, name, task_type, provider, protocol, description, inputs, defaults, options, enabled, builtin, created_at, updated_at)
SELECT
  'doubao-seed-audio-1.0-reference', 'Doubao Seed Audio 1.0 Reference', 'audio', 'volcengine', 'doubao-audio-v3',
  '豆包音频参考音频生成，最多支持 3 段参考音频', '["audio"]',
  '{"providerModel":"seed-audio-1.0","responseFormat":"mp3","sampleRate":24000,"speechRate":0,"loudnessRate":0,"pitchRate":0}',
  '{"responseFormat":["mp3","wav"],"sampleRate":[8000,16000,24000,32000,44100,48000],"speechRate":[-50,-25,0,25,50,100],"loudnessRate":[-50,-25,0,25,50,100],"pitchRate":[-12,-6,0,6,12]}',
  1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM model_definitions WHERE model_key = 'doubao-seed-audio-1.0-reference'
);

INSERT INTO model_definitions
  (model_key, name, task_type, provider, protocol, description, inputs, defaults, options, enabled, builtin, created_at, updated_at)
SELECT
  'doubao-seedream-5-0-lite-260128', 'Seedream 5.0 Lite', 'image', 'volcengine', 'ark-image-v3',
  '文生图与图生图', '["image"]', '{"ratio":"1:1","resolution":"2K"}',
  '{"ratio":["1:1","2:3","3:2","3:4","4:3","9:16","16:9","21:9"],"resolution":["1K","2K"]}',
  1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM model_definitions WHERE model_key = 'doubao-seedream-5-0-lite-260128'
);

INSERT INTO model_definitions
  (model_key, name, task_type, provider, protocol, description, inputs, defaults, options, enabled, builtin, created_at, updated_at)
SELECT
  'doubao-seedance-2-0-260128', 'Seedance 2.0', 'video', 'volcengine', 'ark-video-v3',
  '文生视频与图生视频', '["image"]', '{"ratio":"16:9","resolution":"720p","duration":5}',
  '{"ratio":["16:9","9:16","1:1"],"resolution":["720p","1080p"],"duration":[5,10]}',
  1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM model_definitions WHERE model_key = 'doubao-seedance-2-0-260128'
);
