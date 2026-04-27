/*
 Navicat Premium Data Transfer

 Source Server         : Shinewell
 Source Server Type    : MySQL
 Source Server Version : 80016 (8.0.16)
 Source Host           : localhost:3306
 Source Schema         : cloudque

 Target Server Type    : MySQL
 Target Server Version : 80016 (8.0.16)
 File Encoding         : 65001

 Date: 14/03/2026 11:33:12
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for admin_menu
-- ----------------------------
DROP TABLE IF EXISTS `admin_menu`;
CREATE TABLE `admin_menu`  (
  `id` int(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '菜单ID，主键',
  `parent_id` bigint(20) NULL DEFAULT NULL COMMENT '父菜单id',
  `title` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '菜单标题',
  `sort` bigint(20) NULL DEFAULT NULL COMMENT '排序值',
  `type` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '菜单类型: catalogue, menu',
  `status` bigint(20) NOT NULL COMMENT '状态: 0=停用, 1=启用',
  `icon` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '图标',
  `uri` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '路由路径',
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_admin_menu_title`(`title` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 111 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '菜单表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_menu
-- ----------------------------
INSERT INTO `admin_menu` VALUES (1, 0, '控制台', 1, 'menu', 1, 'AppstoreOutlined', '/dashboard', '2026-02-06 10:35:34.000', '2026-03-03 08:51:54.578');
INSERT INTO `admin_menu` VALUES (2, 0, '操作日志', 2, 'menu', 1, 'AppstoreOutlined', '/operation-log', '2026-02-06 10:35:34.000', '2026-03-09 19:52:48.066');
INSERT INTO `admin_menu` VALUES (3, 0, '用户管理', 3, 'catalogue', 1, 'AppstoreOutlined', '/user', '2026-02-06 10:35:34.000', '2026-02-06 10:35:34.000');
INSERT INTO `admin_menu` VALUES (4, 0, '系统管理', 4, 'catalogue', 1, 'AppstoreOutlined', '/system', '2026-02-06 10:35:34.000', '2026-02-06 10:35:34.000');
INSERT INTO `admin_menu` VALUES (5, 0, '权限管理', 5, 'catalogue', 1, 'AppstoreOutlined', '/permission', '2026-02-06 10:35:34.000', '2026-02-06 10:35:34.000');
INSERT INTO `admin_menu` VALUES (6, 0, '用户功能', 6, 'catalogue', 1, 'AppstoreOutlined', '/feature', '2026-02-06 10:35:34.000', '2026-02-06 10:35:34.000');
INSERT INTO `admin_menu` VALUES (7, 0, '排队系统', 7, 'menu', 1, 'AppstoreOutlined', '/queue', '2026-02-06 11:14:29.000', '2026-03-04 09:05:40.912');
INSERT INTO `admin_menu` VALUES (8, 0, '个人信息', 8, 'catalogue', 1, 'AppstoreOutlined', '/profile', '2026-02-06 10:35:34.000', '2026-02-06 10:35:34.000');
INSERT INTO `admin_menu` VALUES (9, 3, '用户目录', 1, 'catalogue', 1, 'FolderOutlined', '/user/list', '2026-02-06 10:46:47.000', '2026-03-04 09:08:38.727');
INSERT INTO `admin_menu` VALUES (10, 3, '用户信息', 2, 'menu', 1, 'IdcardOutlined', '/user/info', '2026-02-06 10:46:47.000', '2026-02-06 10:46:47.000');
INSERT INTO `admin_menu` VALUES (11, 3, '用户权限', 3, 'menu', 1, 'AppstoreAddOutlined', '/user/permission', '2026-02-06 10:46:47.000', '2026-03-02 08:44:20.948');
INSERT INTO `admin_menu` VALUES (12, 4, '数据展示', 1, 'menu', 1, 'BarChartOutlined', '/system/data', '2026-02-06 10:47:24.000', '2026-02-06 10:47:24.000');
INSERT INTO `admin_menu` VALUES (13, 4, '文件管理', 2, 'menu', 1, 'FolderOpenOutlined', '/system/file', '2026-02-06 10:47:24.000', '2026-02-06 10:47:24.000');
INSERT INTO `admin_menu` VALUES (14, 5, '菜单管理', 1, 'menu', 1, 'UnorderedListOutlined', '/permission/menu', '2026-02-06 10:47:44.000', '2026-02-06 10:47:44.000');
INSERT INTO `admin_menu` VALUES (15, 5, '角色管理', 2, 'menu', 1, 'TeamOutlined', '/permission/role', '2026-02-06 10:47:44.000', '2026-02-06 10:47:44.000');
INSERT INTO `admin_menu` VALUES (16, 5, 'API管理', 3, 'menu', 1, 'ApiOutlined', '/permission/api', '2026-02-06 10:47:44.000', '2026-03-01 16:58:00.622');
INSERT INTO `admin_menu` VALUES (17, 6, '命令窗口', 1, 'menu', 1, 'SwitcherOutlined', '/feature/command', '2026-02-06 10:48:08.000', '2026-02-06 10:48:08.000');
INSERT INTO `admin_menu` VALUES (18, 6, '排队信息', 2, 'menu', 1, 'MenuUnfoldOutlined', '/feature/message', '2026-02-06 10:48:08.000', '2026-02-06 10:48:08.000');
INSERT INTO `admin_menu` VALUES (19, 6, '任务提交', 3, 'menu', 1, 'UpSquareOutlined', '/feature/task', '2026-02-06 10:48:08.000', '2026-02-06 10:48:08.000');
INSERT INTO `admin_menu` VALUES (20, 6, '我的任务', 4, 'menu', 1, 'FileSearchOutlined', '/feature/mytask', '2026-02-06 10:48:08.000', '2026-02-06 10:48:08.000');
INSERT INTO `admin_menu` VALUES (21, 8, '个人资料', 1, 'menu', 1, 'UserOutlined', '/profile/info', '2026-02-06 10:48:21.000', '2026-02-06 10:48:21.000');

-- ----------------------------
-- Table structure for admin_operation_log
-- ----------------------------
DROP TABLE IF EXISTS `admin_operation_log`;
CREATE TABLE `admin_operation_log`  (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `username` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `action_type` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `created_at` datetime(3) NULL DEFAULT NULL,
  `status` bigint(20) NULL DEFAULT 1,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_admin_operation_log_action_type`(`action_type` ASC) USING BTREE,
  INDEX `idx_admin_operation_log_created_at`(`created_at` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 34 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_operation_log
-- ----------------------------

-- ----------------------------
-- Table structure for admin_permissions
-- ----------------------------
DROP TABLE IF EXISTS `admin_permissions`;
CREATE TABLE `admin_permissions`  (
  `id` int(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '权限ID，主键',
  `name` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '权限名称',
  `category` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT 'API类别',
  `slug` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '权限唯一标识',
  `type` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '权限类型',
  `sort` bigint(20) NULL DEFAULT NULL COMMENT 'API排序',
  `status` bigint(20) NULL DEFAULT NULL COMMENT '0-禁用 1-启用',
  `http_method` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT 'API请求方法',
  `http_path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT 'API路径',
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_admin_permissions_slug`(`slug` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1012 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '权限表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_permissions
-- ----------------------------
INSERT INTO `admin_permissions` VALUES (202, '重启服务器', '控制台', 'cloud:dashboard:restart', 'dashboard', 3, 1, 'GET', '/api/v1/admin/restart', '2026-02-10 21:03:19.000', NULL);
INSERT INTO `admin_permissions` VALUES (203, '查看用户目录', '用户目录', 'cloud:user:catalogue:list', 'user', 1, 1, 'GET', '/api/v1/directories/list-home', '2026-02-10 20:48:13.000', NULL);
INSERT INTO `admin_permissions` VALUES (204, '查看用户信息', '用户信息', 'cloud:user:list', 'user', 1, 1, 'GET', '/api/v1/user/list', '2026-02-10 20:48:14.000', NULL);
INSERT INTO `admin_permissions` VALUES (205, '添加用户', '用户信息', 'cloud:user:add', 'user', 2, 1, 'POST', '/api/v1/admin/users', '2026-02-06 08:48:20.000', NULL);
INSERT INTO `admin_permissions` VALUES (206, '编辑用户', '用户信息', 'cloud:user:update', 'user', 3, 1, 'PUT', '/api/v1/admin/users/:id', '2026-02-06 08:48:22.000', NULL);
INSERT INTO `admin_permissions` VALUES (207, '删除用户', '用户信息', 'cloud:user:delete', 'user', 4, 1, 'DELETE', '/api/v1/admin/users/:id', '2026-02-06 08:48:24.000', NULL);
INSERT INTO `admin_permissions` VALUES (210, '查看系统数据', '数据展示', 'cloud:system:list', 'system', 1, 1, 'GET', '/api/v1/system', '2026-02-10 20:56:36.000', NULL);
INSERT INTO `admin_permissions` VALUES (211, '查看文件列表(user)', '文件管理', 'cloud:file:user:list', 'file', 11, 1, 'GET', '/api/v1/files/list', '2026-02-10 21:11:10.000', NULL);
INSERT INTO `admin_permissions` VALUES (212, '文件上传(user)', '文件管理', 'cloud:file:user:upload', 'file', 12, 1, 'POST', '/api/v1/files/upload', '2026-02-10 21:12:19.000', NULL);
INSERT INTO `admin_permissions` VALUES (213, '文件下载(user)', '文件管理', 'cloud:file:user:download', 'file', 13, 1, 'GET', '/api/v1/files/download', '2026-02-10 21:14:33.000', NULL);
INSERT INTO `admin_permissions` VALUES (214, '删除文件(user)', '文件管理', 'cloud:file:user:delete', 'file', 14, 1, 'DELETE', '/api/v1/files/delete', '2026-02-10 21:24:07.000', NULL);
INSERT INTO `admin_permissions` VALUES (215, '解压文件(user)', '文件管理', 'cloud:file:user:unzip', 'file', 15, 1, 'POST', '/api/v1/files/unzip', '2026-02-10 21:29:52.000', NULL);
INSERT INTO `admin_permissions` VALUES (216, '分页查看菜单信息', '菜单管理', 'cloud:authority:menu:list', 'menu', 1, 1, 'GET', '/api/v1/permissionManage/menus/page', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (217, '新增菜单', '菜单管理', 'cloud:authority:menu:add', 'menu', 2, 1, 'POST', '/api/v1/permissionManage/menus', '2026-02-11 09:56:00.000', '2026-03-06 10:38:18.794');
INSERT INTO `admin_permissions` VALUES (218, '编辑菜单', '菜单管理', 'cloud:authority:menu:update', 'menu', 3, 1, 'PUT', '/api/v1/permissionManage/menus', '2026-02-11 09:57:10.000', NULL);
INSERT INTO `admin_permissions` VALUES (219, '删除菜单', '菜单管理', 'cloud:authority:menu:delete', 'menu', 4, 1, 'DELETE', '/api/v1/permissionManage/menus/:id', '2026-02-11 10:58:52.000', NULL);
INSERT INTO `admin_permissions` VALUES (220, '分页查看角色信息', '角色管理', 'cloud:authority:role:list', 'role', 1, 1, 'GET', '/api/v1/permissionManage/roles/page', '2026-02-11 11:00:13.000', NULL);
INSERT INTO `admin_permissions` VALUES (221, '新增角色', '角色管理', 'cloud:authority:role:add', 'role', 2, 1, 'POST', '/api/v1/permissionManage/roles', '2026-02-11 11:01:35.000', NULL);
INSERT INTO `admin_permissions` VALUES (222, '编辑角色', '角色管理', 'cloud:authority:role:update', 'role', 3, 1, 'PUT', '/api/v1/permissionManage/roles', '2026-02-11 11:02:54.000', NULL);
INSERT INTO `admin_permissions` VALUES (223, '删除角色', '角色管理', 'cloud:authority:role:delete', 'role', 4, 1, 'DELETE', '/api/v1/permissionManage/roles/:id', '2026-02-11 11:04:25.000', NULL);
INSERT INTO `admin_permissions` VALUES (224, '查看角色权限', '角色管理', 'cloud:authority:role:permission:list', 'role', 6, 1, 'GET', '/api/v1/permissionManage/roles/role_perm/:id', '2026-02-11 11:07:13.000', NULL);
INSERT INTO `admin_permissions` VALUES (225, '修改角色权限', '角色管理', 'cloud:authority:role:permission:update', 'role', 7, 1, 'PUT', '/api/v1/permissionManage/roles/role_perm/:role_id', '2026-02-11 11:08:32.000', NULL);
INSERT INTO `admin_permissions` VALUES (226, '分页查看API信息', 'API管理', 'cloud:authority:api:list', 'api', 1, 1, 'GET', '/api/v1/permissionManage/API/page', '2026-02-11 11:10:59.000', NULL);
INSERT INTO `admin_permissions` VALUES (227, '新增API', 'API管理', 'cloud:authority:api:add', 'api', 2, 1, 'POST', '/api/v1/permissionManage/API', '2026-02-11 11:12:48.000', NULL);
INSERT INTO `admin_permissions` VALUES (228, '编辑API', 'API管理', 'cloud:authority:api:update', 'api', 3, 1, 'PUT', '/api/v1/permissionManage/API', '2026-02-11 11:14:11.000', NULL);
INSERT INTO `admin_permissions` VALUES (229, '删除API', 'API管理', 'cloud:authority:api:delete', 'api', 4, 1, 'DELETE', '/api/v1/permissionManage/API/:id', '2026-02-11 11:15:25.000', NULL);
INSERT INTO `admin_permissions` VALUES (230, '命令窗口(user)', '命令窗口', 'cloud:terminal:user:ws', 'feature', 2, 1, 'GET', '/api/v1/terminal/ws', '2026-02-10 20:48:06.000', NULL);
INSERT INTO `admin_permissions` VALUES (231, '查看我的排队信息', '排队信息', 'cloud:feature:task:list', 'feature', 1, 1, 'GET', '/api/v1/job', '2026-02-11 11:22:05.000', NULL);
INSERT INTO `admin_permissions` VALUES (232, '取消我的排队任务', '排队信息', 'cloud:feature:task:cancel', 'feature', 2, 1, 'DELETE', '/api/v1/job/:id', '2026-02-11 11:26:47.000', NULL);
INSERT INTO `admin_permissions` VALUES (233, '提交我的任务', '任务提交', 'cloud:feature:task:submit', 'feature', 1, 1, 'POST', '/api/v1/job', '2026-02-11 11:29:59.000', NULL);
INSERT INTO `admin_permissions` VALUES (234, '查看我的任务', '我的任务', 'cloud:feature:mytask:list', 'feature', 1, 1, 'GET', '/api/v1/job/wait', '2026-02-11 11:32:35.000', NULL);
INSERT INTO `admin_permissions` VALUES (235, '查看所有排队列表', '排队系统', 'cloud:queue:list', 'queue', 1, 1, 'GET', '/api/v1/queue', '2026-02-10 20:48:08.000', NULL);
INSERT INTO `admin_permissions` VALUES (236, '修改排队顺序', '排队系统', 'cloud:queue:move', 'queue', 2, 1, 'POST', '/api/v1/queue', '2026-02-11 11:58:39.000', NULL);
INSERT INTO `admin_permissions` VALUES (237, '删除排队任务', '排队系统', 'cloud:queue:delete', 'queue', 3, 1, 'DELETE', '/api/v1/queue/:jobId', '2026-02-06 09:06:43.278', '2026-02-11 12:26:26.000');
INSERT INTO `admin_permissions` VALUES (238, '查看个人资料', '个人资料', 'cloud:personal:info', 'personal', 1, 1, 'GET', '/api/v1/user/profile', '2026-02-11 11:44:04.000', NULL);
INSERT INTO `admin_permissions` VALUES (239, '修改个人用户名', '个人资料', 'cloud:personal:update', 'personal', 2, 1, 'PUT', '/api/v1/user/profile', '2026-02-11 11:45:40.000', NULL);
INSERT INTO `admin_permissions` VALUES (240, '查看用户操作信息', '操作日志', 'cloud:log:user:list', 'log', 1, 1, 'GET', '/api/v1/operationLogs/userLog/list', '2026-02-11 12:00:32.000', NULL);
INSERT INTO `admin_permissions` VALUES (241, '查看管理员操作日志', '操作日志', 'cloud:log:admin:list', 'log', 2, 1, 'GET', '/api/v1/operationLogs/adminLog/list', '2026-02-11 21:34:21.000', NULL);
INSERT INTO `admin_permissions` VALUES (242, '修改密码', '个人资料', 'cloud:personal:password', 'personal', 2, 1, 'PUT', '/api/v1/user/password', '2026-02-25 19:26:29.000', NULL);
INSERT INTO `admin_permissions` VALUES (243, '关闭服务器', '控制台', 'cloud:dashboard:shutdown', 'dashboard', 3, 1, 'GET', '/api/v1/admin/shutdown', '2026-02-25 19:23:01.000', NULL);
INSERT INTO `admin_permissions` VALUES (244, '修改头像', '个人资料', 'cloud:personal:avatar', 'personal', 2, 1, 'POST', '/api/v1/user/avatar', '2026-02-25 19:25:10.000', NULL);
INSERT INTO `admin_permissions` VALUES (245, '任务统计', '控制台', 'cloud:dashboard:job', 'dashboard', 1, 1, 'GET', '/api/v1/job/stats', '2026-02-26 09:37:34.000', NULL);
INSERT INTO `admin_permissions` VALUES (246, '获取显卡信息', '任务提交', 'cloud:dashboard:gpu', 'dashboard', 2, 1, 'GET', '/api/v1/job/gpus', '2026-02-26 09:38:51.000', NULL);
INSERT INTO `admin_permissions` VALUES (253, '查看目录磁盘占用', '用户目录', 'cloud:user:catalogue:calculate-usage', 'user', 2, 1, 'GET', '/api/v1/directories/calculate-usage', '2026-02-10 21:29:52.000', NULL);
INSERT INTO `admin_permissions` VALUES (255, '查看菜单详情', '菜单管理', 'cloud:authority:menu:view', 'menu', 1, 1, 'GET', '/api/v1/permissionManage/menus/:id', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (256, '查看角色详情', '角色管理', 'cloud:authority:role:view', 'role', 1, 1, 'GET', '/api/v1/permissionManage/roles/:id', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (257, '查看API详情', 'API管理', 'cloud:authority:api:view', 'api   ', 1, 1, 'GET', '/api/v1/permissionManage/API/:id', NULL, '2026-03-04 15:40:34.999');
INSERT INTO `admin_permissions` VALUES (258, '批量删除角色', '角色管理', 'cloud:authority:role:delete:batch', 'role', 5, 1, 'DELETE', '/api/v1/permissionManage/roles/batch', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (259, '批量删除菜单', '菜单管理', 'cloud:authority:menu:delete:batch', 'menu', 5, 1, 'DELETE', '/api/v1/permissionManage/menus/batch', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (260, '批量删除API', 'API管理', 'cloud:authority:api:delete:batch', 'api', 5, 1, 'DELETE', '/api/v1/permissionManage/API/batch', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (261, '查看文件列表(system)', '文件管理', 'cloud:file:root:list', 'file', 1, 1, 'GET', '/api/v1/files/list', '2026-02-10 21:11:10.000', NULL);
INSERT INTO `admin_permissions` VALUES (262, '文件上传(system)', '文件管理', 'cloud:file:root:upload', 'file', 2, 1, 'POST', '/api/v1/files/upload', '2026-02-10 21:12:19.000', NULL);
INSERT INTO `admin_permissions` VALUES (263, '文件下载(system)', '文件管理', 'cloud:file:root:download', 'file', 3, 1, 'GET', '/api/v1/files/download', '2026-02-10 21:14:33.000', NULL);
INSERT INTO `admin_permissions` VALUES (264, '删除文件(system)', '文件管理', 'cloud:file:root:delete', 'file', 4, 1, 'DELETE', '/api/v1/files/delete', '2026-02-10 21:24:07.000', NULL);
INSERT INTO `admin_permissions` VALUES (265, '解压文件(system)', '文件管理', 'cloud:file:root:unzip', 'file', 5, 1, 'POST', '/api/v1/files/unzip', '2026-02-10 21:29:52.000', NULL);
INSERT INTO `admin_permissions` VALUES (266, '命令窗口(root)', '命令窗口', 'cloud:terminal:root:ws', 'feature', 1, 1, 'GET', '/api/v1/terminal/ws', '2026-02-10 20:48:06.000', NULL);
INSERT INTO `admin_permissions` VALUES (267, '获取所有角色', '用户权限', 'cloud:user:role', 'user', 1, 1, 'GET', '/api/v1/admin/roles/simple', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (268, '获取菜单树跟权限', '用户权限', 'cloud:user:tree', 'user', 2, 1, 'GET', '/api/v1/user/menu-permission', NULL, NULL);
INSERT INTO `admin_permissions` VALUES (269, '查看文件上传进度', '文件管理', 'cloud:file:uploarprogress', 'file', 6, 1, 'GET', '/api/v1/files/upload/progress', NULL, NULL);

-- ----------------------------
-- Table structure for admin_role_menu
-- ----------------------------
DROP TABLE IF EXISTS `admin_role_menu`;
CREATE TABLE `admin_role_menu`  (
  `role_id` int(10) UNSIGNED NOT NULL COMMENT '角色ID',
  `menu_id` int(10) UNSIGNED NOT NULL COMMENT '菜单ID',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`role_id`, `menu_id`) USING BTREE,
  INDEX `idx_menu_id`(`menu_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色-菜单多对多关联表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_role_menu
-- ----------------------------
INSERT INTO `admin_role_menu` VALUES (1, 1, '2026-03-04 08:54:43', '2026-03-04 08:54:43');
INSERT INTO `admin_role_menu` VALUES (1, 2, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 3, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 4, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 5, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 6, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 7, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 8, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 9, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 10, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 11, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 12, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 13, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 14, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 15, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 16, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 17, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 18, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 19, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 20, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 21, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (1, 94, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_menu` VALUES (2, 4, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 6, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 8, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 13, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 17, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 18, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 19, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 20, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_menu` VALUES (2, 21, '2026-03-12 15:57:17', '2026-03-12 15:57:17');

-- ----------------------------
-- Table structure for admin_role_permissions
-- ----------------------------
DROP TABLE IF EXISTS `admin_role_permissions`;
CREATE TABLE `admin_role_permissions`  (
  `role_id` int(10) UNSIGNED NOT NULL COMMENT '角色ID',
  `permission_id` int(10) UNSIGNED NOT NULL COMMENT '权限ID',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`role_id`, `permission_id`) USING BTREE,
  INDEX `idx_permission_id`(`permission_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色-权限多对多关联表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_role_permissions
-- ----------------------------
INSERT INTO `admin_role_permissions` VALUES (1, 202, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 203, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 204, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 205, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 206, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 207, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 210, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 211, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 212, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 213, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 214, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 215, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 216, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 217, '2026-03-04 16:57:34', '2026-03-04 16:57:31');
INSERT INTO `admin_role_permissions` VALUES (1, 218, '2026-03-04 16:57:39', '2026-03-04 16:57:41');
INSERT INTO `admin_role_permissions` VALUES (1, 219, '2026-03-04 16:57:50', '2026-03-04 16:57:53');
INSERT INTO `admin_role_permissions` VALUES (1, 220, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 221, '2026-03-04 08:58:22', '2026-03-04 08:58:49');
INSERT INTO `admin_role_permissions` VALUES (1, 222, '2026-03-04 16:58:55', '2026-03-04 16:58:56');
INSERT INTO `admin_role_permissions` VALUES (1, 223, '2026-03-04 16:59:02', '2026-03-04 16:59:04');
INSERT INTO `admin_role_permissions` VALUES (1, 224, '2026-03-04 16:59:10', '2026-03-04 16:59:12');
INSERT INTO `admin_role_permissions` VALUES (1, 225, '2026-03-04 16:59:21', '2026-03-04 16:59:22');
INSERT INTO `admin_role_permissions` VALUES (1, 226, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 227, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 228, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 229, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 230, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 231, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 232, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 233, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 234, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 235, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 236, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 237, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 238, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 239, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 240, '2026-03-04 17:00:20', '2026-03-04 17:00:20');
INSERT INTO `admin_role_permissions` VALUES (1, 241, '2026-03-04 17:00:26', '2026-03-04 17:00:27');
INSERT INTO `admin_role_permissions` VALUES (1, 242, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 243, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 244, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 245, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 246, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 253, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 255, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 256, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 257, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 258, '2026-03-04 17:00:55', '2026-03-04 17:00:56');
INSERT INTO `admin_role_permissions` VALUES (1, 259, '2026-03-04 17:01:03', '2026-03-04 17:01:04');
INSERT INTO `admin_role_permissions` VALUES (1, 260, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 261, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 262, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 263, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 264, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 265, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 266, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 267, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 268, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (1, 269, '2026-03-04 15:39:54', '2026-03-04 15:39:54');
INSERT INTO `admin_role_permissions` VALUES (2, 211, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 212, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 213, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 214, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 215, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 230, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 231, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 232, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 233, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 234, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 238, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 239, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 242, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 244, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 246, '2026-03-12 15:57:17', '2026-03-12 15:57:17');
INSERT INTO `admin_role_permissions` VALUES (2, 269, '2026-03-12 15:57:17', '2026-03-12 15:57:17');

-- ----------------------------
-- Table structure for admin_role_users
-- ----------------------------
DROP TABLE IF EXISTS `admin_role_users`;
CREATE TABLE `admin_role_users`  (
  `role_id` int(10) UNSIGNED NOT NULL COMMENT '角色ID',
  `user_id` int(10) UNSIGNED NOT NULL COMMENT '用户ID',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`role_id`, `user_id`) USING BTREE,
  INDEX `idx_user_id`(`user_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色-用户多对多关联表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_role_users
-- ----------------------------
INSERT INTO `admin_role_users` VALUES (1, 1, '2026-03-14 11:17:05', '2026-03-14 11:17:07');

-- ----------------------------
-- Table structure for admin_roles
-- ----------------------------
DROP TABLE IF EXISTS `admin_roles`;
CREATE TABLE `admin_roles`  (
  `id` int(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色ID，主键',
  `name` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '角色名称',
  `status` bigint(20) NULL DEFAULT NULL COMMENT '0-禁用 1-启用',
  `slug` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '角色唯一标识',
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  `deleted_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_admin_roles_slug`(`slug` ASC) USING BTREE,
  UNIQUE INDEX `idx_admin_roles_name`(`name` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 28 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_roles
-- ----------------------------
INSERT INTO `admin_roles` VALUES (1, '超级管理员', 1, 'admin', '2026-02-05 16:22:07.017', '2026-03-02 16:20:47.841', NULL);
INSERT INTO `admin_roles` VALUES (2, '普通用户', 1, 'user', '2026-02-05 16:41:59.377', '2026-02-05 16:41:59.377', NULL);

-- ----------------------------
-- Table structure for admin_users
-- ----------------------------
DROP TABLE IF EXISTS `admin_users`;
CREATE TABLE `admin_users`  (
  `id` int(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID，主键',
  `username` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '登录账号',
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '加密后的密码',
  `avatar` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL COMMENT '用户头像(Base64格式)',
  `email` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '邮箱',
  `status` bigint(20) NULL DEFAULT 1 COMMENT '账号状态',
  `priority` bigint(20) NULL DEFAULT 1 COMMENT '用户优先级',
  `multi_training` bigint(20) NULL DEFAULT 0 COMMENT '多卡训练',
  `cross_server` bigint(20) NULL DEFAULT 0 COMMENT '跨服务器调度',
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_admin_users_username`(`username` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 92 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_users
-- ----------------------------
INSERT INTO `admin_users` VALUES (1, 'super', '$2a$10$sKy2RUvStvL92iPDzshp9O6/vY9v9Kfcuats.ltv0wem48P2IVJYa', '', '149930822@qq.com', 1, 2, 1, 1, '2026-02-27 21:51:33.294', '2026-03-12 15:26:14.834');

-- ----------------------------
-- Table structure for admin_users_permission
-- ----------------------------
DROP TABLE IF EXISTS `admin_users_permission`;
CREATE TABLE `admin_users_permission`  (
  `user_id` int(11) NOT NULL COMMENT '用户id',
  `permission_id` int(11) NOT NULL COMMENT '权限id',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`, `permission_id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of admin_users_permission
-- ----------------------------

-- ----------------------------
-- Table structure for base_entities
-- ----------------------------
DROP TABLE IF EXISTS `base_entities`;
CREATE TABLE `base_entities`  (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of base_entities
-- ----------------------------

-- ----------------------------
-- Table structure for gpu_cards
-- ----------------------------
DROP TABLE IF EXISTS `gpu_cards`;
CREATE TABLE `gpu_cards`  (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `uuid` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `name` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `index` bigint(20) NOT NULL,
  `status` tinyint(4) NOT NULL DEFAULT 0,
  `current_job_id` bigint(20) NULL DEFAULT NULL,
  `gpu_type` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL,
  `memory` bigint(20) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_gpu_cards_name`(`name` ASC) USING BTREE,
  UNIQUE INDEX `idx_gpu_cards_uuid`(`uuid` ASC) USING BTREE,
  INDEX `idx_gpu_cards_current_job_id`(`current_job_id` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 3035 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of gpu_cards
-- ----------------------------

-- ----------------------------
-- Table structure for jobs
-- ----------------------------
DROP TABLE IF EXISTS `jobs`;
CREATE TABLE `jobs`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务ID',
  `name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL,
  `user_id` bigint(20) NOT NULL,
  `status` bigint(20) NULL DEFAULT NULL,
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `finished_at` datetime(3) NULL DEFAULT NULL,
  `sug` int(11) NULL DEFAULT 0 COMMENT '标识是否被手动插队',
  `file_path` varchar(191) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `gpu_ids` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `started_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_user_id`(`user_id` ASC) USING BTREE,
  INDEX `idx_status`(`status` ASC) USING BTREE,
  INDEX `idx_job_create_time`(`created_at` ASC) USING BTREE,
  INDEX `idx_jobs_user_id`(`user_id` ASC) USING BTREE,
  INDEX `idx_jobs_file_path`(`file_path` ASC) USING BTREE,
  INDEX `idx_jobs_status`(`status` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 405 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '训练任务/作业表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of jobs
-- ----------------------------

-- ----------------------------
-- Table structure for operation_logs
-- ----------------------------
DROP TABLE IF EXISTS `operation_logs`;
CREATE TABLE `operation_logs`  (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `username` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `method` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `request_data` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL,
  `action_type` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `status` bigint(20) NULL DEFAULT 1,
  `error_message` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL,
  `created_at` datetime(3) NULL DEFAULT NULL,
  `updated_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_operation_logs_username`(`username` ASC) USING BTREE,
  INDEX `idx_operation_logs_action_type`(`action_type` ASC) USING BTREE,
  INDEX `idx_operation_logs_created_at`(`created_at` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 6031 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of operation_logs
-- ----------------------------

-- ----------------------------
-- Table structure for processes
-- ----------------------------
DROP TABLE IF EXISTS `processes`;
CREATE TABLE `processes`  (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `pid` bigint(20) NOT NULL,
  `card_id` bigint(20) NOT NULL,
  `job_id` bigint(20) NOT NULL,
  `created_at` datetime(3) NULL DEFAULT NULL,
  `ended_at` datetime(3) NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_processes_p_id`(`pid` ASC) USING BTREE,
  INDEX `idx_processes_card_id`(`card_id` ASC) USING BTREE,
  INDEX `idx_processes_job_id`(`job_id` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 281 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of processes
-- ----------------------------

-- ----------------------------
-- Table structure for server
-- ----------------------------
DROP TABLE IF EXISTS `server`;
CREATE TABLE `server`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '节点ID',
  `hostname` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '主机名',
  `ip_address` varchar(45) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT 'IP地址',
  `status` tinyint(3) UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1-在线, 0-离线',
  `total_disk` bigint(20) UNSIGNED NULL DEFAULT NULL COMMENT '总磁盘空间 (MB)',
  `total_memory` bigint(20) UNSIGNED NULL DEFAULT NULL COMMENT '总内存 (MB)',
  `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '节点描述',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_node_status`(`status` ASC) USING BTREE,
  INDEX `idx_node_ip`(`ip_address` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '计算节点/服务器表' ROW_FORMAT = DYNAMIC;

-- ----------------------------
-- Records of server
-- ----------------------------

SET FOREIGN_KEY_CHECKS = 1;
