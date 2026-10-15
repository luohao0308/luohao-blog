CREATE TABLE IF NOT EXISTS `projects` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `slug` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `name` varchar(128) COLLATE utf8mb4_bin NOT NULL,
  `summary` longtext COLLATE utf8mb4_bin,
  `description` longtext COLLATE utf8mb4_bin,
  `stack` json NOT NULL,
  `role` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `repo_url` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `live_url` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `emoji` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `year` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `featured` boolean NOT NULL DEFAULT FALSE,
  `cover_url` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `screenshots` json NOT NULL,
  `problem` longtext COLLATE utf8mb4_bin,
  `approach` longtext COLLATE utf8mb4_bin,
  `outcome` longtext COLLATE utf8mb4_bin,
  `published` boolean NOT NULL DEFAULT FALSE,
  `sort` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `project_slug` (`slug`),
  KEY `project_updated_at` (`updated_at`),
  KEY `project_published_sort_slug` (`published`, `sort`, `slug`),
  KEY `project_sort` (`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS `abouts` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `singleton_key` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT 'main',
  `name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `headline` longtext COLLATE utf8mb4_bin,
  `intro` longtext COLLATE utf8mb4_bin,
  `profile_image` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `current_focus` longtext COLLATE utf8mb4_bin,
  `skills` json NOT NULL,
  `timeline` json NOT NULL,
  `contacts` json NOT NULL,
  `representative_links` json NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `about_singleton_key` (`singleton_key`),
  KEY `about_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

INSERT IGNORE INTO `projects` (`id`,`created_at`,`updated_at`,`slug`,`name`,`summary`,`description`,`stack`,`role`,`repo_url`,`live_url`,`emoji`,`year`,`featured`,`cover_url`,`screenshots`,`problem`,`approach`,`outcome`,`published`,`sort`) VALUES
('00000000-0000-7000-8000-000000000101',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'luohao-blog','luohao-blog','本站：Go + Kratos 后端、Nuxt 4 SSR 前端的全栈个人站','单体仓库全栈项目：Kratos 承载 REST/gRPC 契约与文章渲染管道（goldmark + chroma），Nuxt 4 负责 SSR 与 BFF 代理；开发全流程由自研 dev-workflow 治理（fail-closed 交付门禁、版本化迁移、契约索引）。本站的所有功能迭代都以文章形式记录在博客里。',JSON_ARRAY('Go','Kratos','ent','Nuxt 4','Tailwind CSS','MySQL','Redis','Elasticsearch'),'独立设计与开发','https://github.com/luohao0308/luohao-blog','','🛠️','2026',TRUE,'',JSON_ARRAY(),'','','',TRUE,10),
('00000000-0000-7000-8000-000000000102',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'dev-workflow','dev-workflow','可分发的 AI 协作开发流程：把大厂工程治理装进任意仓库','一套安装到任意代码仓库的 AI 协作开发流程：项目画像、契约治理、交付门禁（push/PR/merge 全部 fail-closed 校验）、任务上下文与工作日志。本站从建仓到每个 PR 的合并都由它把守。',JSON_ARRAY('Bash','Python','PowerShell','AI Workflow'),'作者与维护者','https://github.com/luohao0308/dev-workflow','','⚙️','2026',TRUE,'',JSON_ARRAY(),'','','',TRUE,20),
('00000000-0000-7000-8000-000000000103',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'job-application-workbench','job-application-workbench','求职投递工作台：岗位聚合、匹配度排序与投递状态管理','Vue + Node 全栈工具：聚合多渠道岗位、按个人画像做匹配度排序、跟踪投递状态流转。M1 之前的主要练手项目，vue-workflow 治理模式的发源地。',JSON_ARRAY('Vue 3','Node.js','Vite'),'独立开发','','','📋','2025',FALSE,'',JSON_ARRAY(),'','','',TRUE,30);

INSERT IGNORE INTO `abouts` (`id`,`created_at`,`updated_at`,`singleton_key`,`name`,`headline`,`intro`,`profile_image`,`current_focus`,`skills`,`timeline`,`contacts`,`representative_links`) VALUES
('00000000-0000-7000-8000-000000000013',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'main','','全栈工程师 · Go / Vue 双栈 · AI Agent 工程实践者。','相信工程治理和自动化流程是个人开发者的杠杆。','','',
JSON_ARRAY(JSON_OBJECT('group','后端','items',JSON_ARRAY('Go','Kratos','ent','gRPC / protobuf','MySQL','Redis','Elasticsearch')),JSON_OBJECT('group','前端','items',JSON_ARRAY('Vue 3','Nuxt 4','TypeScript','Tailwind CSS','Vite')),JSON_OBJECT('group','工程化','items',JSON_ARRAY('Docker','GitHub Actions','golang-migrate','Prometheus / Grafana','AI 协作开发流程'))),
JSON_ARRAY(JSON_OBJECT('year','2026','text','搭建 luohao-blog：全栈个人站 + AI 交付治理实践场，所有迭代过程写成文章。'),JSON_OBJECT('year','2026','text','开源 dev-workflow：把大厂工程治理（契约/门禁/可观测）做成可安装的 AI 协作流程。'),JSON_OBJECT('year','2025','text','深耕全栈：Vue / Go 双栈项目、求职工作台等工具型产品。')),
JSON_ARRAY(JSON_OBJECT('label','GitHub','href','https://github.com/luohao0308','text','github.com/luohao0308'),JSON_OBJECT('label','邮箱','href','mailto:2429260713@qq.com','text','2429260713@qq.com')),
JSON_ARRAY(JSON_OBJECT('label','luohao-blog','href','https://github.com/luohao0308/luohao-blog'),JSON_OBJECT('label','dev-workflow','href','https://github.com/luohao0308/dev-workflow')));
