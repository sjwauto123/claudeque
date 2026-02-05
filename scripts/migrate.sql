-- CloudQue 数据库建表脚本

-- 创建数据库
CREATE DATABASE IF NOT EXISTS cloudque DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE cloudque;

-- 角色表
CREATE TABLE IF NOT EXISTS `admin_roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL COMMENT '角色名称',
  `status` tinyint DEFAULT NULL COMMENT '角色状态 0-禁用 1-启用',
  `slug` varchar(50) NOT NULL COMMENT '角色唯一标识',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_slug` (`slug`),
  UNIQUE KEY `idx_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `username` varchar(190) NOT NULL COMMENT '登录账号，唯一',
  `password` varchar(60) NOT NULL COMMENT '加密后的密码',
  `role` tinyint NOT NULL DEFAULT 1 COMMENT '用户角色 1-普通用户 2-管理员',
  `avatar` varchar(191) DEFAULT '' COMMENT '头像URL',
  `email` varchar(255) DEFAULT NULL COMMENT '邮箱',
  `remember_token` varchar(100) DEFAULT '' COMMENT '记住我Token',
  `status` tinyint NOT NULL COMMENT '账号状态',
  `priority` tinyint NOT NULL DEFAULT 1 COMMENT '用户优先级 1-低 2-高',
  `multi_training` tinyint NOT NULL DEFAULT 0 COMMENT '多卡训练 0-否 1-是',
  `cross_server` tinyint NOT NULL DEFAULT 0 COMMENT '跨服务器调度 0-否 1-是',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_username` (`username`),
  KEY `idx_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 操作日志表
CREATE TABLE IF NOT EXISTS `operation_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '日志ID',
  `user_id` int unsigned NOT NULL COMMENT '操作用户ID，关联users.id',
  `action_type` varchar(50) NOT NULL COMMENT '操作类型 (Login, SubmitJob, CancelJob, DeleteFile...)',
  `description` text COMMENT '操作详情描述',
  `ip_address` varchar(45) NOT NULL COMMENT '操作IP地址',
  `created_at` timestamp DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  `updated_at` timestamp DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_action_type` (`action_type`),
  KEY `idx_operation_time` (`created_at`),
  KEY `idx_user_operation` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='用户操作日志表';
