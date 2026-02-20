3. 游戏核心逻辑 (Game Mechanics)
3.1 地图与视野 (Map & FOV)

    地图：矩形区域，边界不可逾越。尺寸 (MapW, MapH) 由房主在创建房间时配置。

    视野 (Fog of War)：

        客户端只渲染 Player.Pos 周围 ViewRadius 矩形区域内的实体。

        服务端优化：使用 AOI (Area of Interest) 算法（如九宫格或四叉树）进行广播筛选，只向玩家发送视野内的数据，节省带宽。

3.2 角色控制 (Controls)

    移动：W/S/A/D 控制上下左右移动。客户端进行预表现（Prediction），服务端进行权威验证与位置纠正（Reconciliation）。

    攻击 (蓄力光柱)：

        Input：按下方向键（↑/↓/←/→）开始蓄力，松开按键发射。

        状态机：Idle -> Charging (记录 start_time) -> Firing (计算 duration) -> Cooldown。

        光柱属性：

            长度 L = base_len + k1 * charge_time (有上限)。

            伤害 D = base_dmg + k2 * charge_time (有上限)。

            判定：光柱为瞬时或短时持续的矩形碰撞盒（OBB）。

3.3 游戏界面布局 (UI Layout)

界面采用 CSS Grid 或 Flex 布局，分为三部分：

    左侧 (Charge Panel)：

        包含一个垂直进度条组件。

        逻辑：监听玩家 Charging 状态，根据 (Now - StartTime) 动态渲染高度。

    右侧/中央 (Game Viewport)：

        <canvas> 元素，由 Pixi.js 接管。

        渲染：背景地图、其他玩家 Sprite、自己 Sprite、光柱特效、伤害数字。

    下方 (Info Panel)：

        玩家列表：表格显示 Name | Kills | Deaths | Ping。

        个人状态：HP 数值/血条，当前坐标（Debug用）