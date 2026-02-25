# 终端 WebSocket 接口文档补充（符合 API3.0.md）

## 建立连接

```
GET /api/terminal/connect?token=<JWT>
```
或使用 Header：
```
Authorization: Bearer <JWT>
```

### 说明
- 长连接，协议升级至 WebSocket。
- 登录成功后请立即调用，保持心跳即可维持后端 SSH 会话。

## WebSocket 消息格式（JSON）

| type    | 必填 | 字段               | 说明                                                         |
|---------|------|--------------------|--------------------------------------------------------------|
| input   | 是   | data `string`      | 发送到远端 shell 的键盘输入                                   |
| output  | 系统 | data `string`      | 服务器返回的终端输出（stdout/stderr 合并）                   |
| resize  | 是   | cols `int`, rows `int` | 终端窗口大小（列/行）                                         |
| ping    | 无   | –                  | 前端主动心跳                                                 |
| pong    | 系统 | –                  | 服务器心跳响应                                               |
| close   | 可选 | –                  | 前端主动关闭会话                                             |

> 纯文本兼容
>
> 如前端仍直接发送纯文本（无 JSON 封装），后端依旧会把该数据写入远端 stdin。

## 示例（xterm.js in JS）
```js
const ws = new WebSocket("wss://example.com/api/terminal/connect?token=" + token);

ws.onopen = () => {
  // 心跳
  setInterval(()=>ws.send(JSON.stringify({type:"ping"})), 10000);
};

term.onData(data => {
  ws.send(JSON.stringify({type:"input", data}));
});

term.onResize(({cols, rows}) => {
  ws.send(JSON.stringify({type:"resize", cols, rows}));
});

ws.onmessage = ({data}) => {
  const msg = JSON.parse(data);
  if (msg.type === "output") term.write(msg.data);
};
```

## 错误码 / 关闭语义
- WebSocket 关闭码 `4001`：鉴权失败
- WebSocket 关闭码 `4002`：SSH 会话不存在或已过期
- 正常退出：服务端发送 `close` 后主动 `Close()`，浏览器应销毁终端实例。
