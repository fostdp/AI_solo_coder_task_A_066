(function (global) {
    var NS = global.CoolingPlatform = global.CoolingPlatform || {};

    var BG_COLOR = '#0a1628';
    var TEXT_COLOR = '#90a4ae';
    var GRID_COLOR = '#1e3a5f';
    var GREEN = '#00e676';
    var YELLOW = '#ffd600';
    var RED = '#ff1744';
    var BLUE = '#2196f3';
    var ORANGE = '#ff9800';
    var CYAN = '#00bcd4';

    function setupCanvas(canvas) {
        var dpr = window.devicePixelRatio || 1;
        var rect = canvas.getBoundingClientRect();
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        var ctx = canvas.getContext('2d');
        ctx.scale(dpr, dpr);
        return { ctx: ctx, w: rect.width, h: rect.height, dpr: dpr };
    }

    function formatTime(ts) {
        var d = new Date(ts);
        var hh = ('0' + d.getHours()).slice(-2);
        var mm = ('0' + d.getMinutes()).slice(-2);
        return hh + ':' + mm;
    }

    function pueLineColor(val) {
        if (val < 1.4) return GREEN;
        if (val < 1.5) return YELLOW;
        return RED;
    }

    function Charts() {}

    Charts.prototype.drawPUETrend = function (canvasId, data) {
        var canvas = document.getElementById(canvasId);
        if (!canvas || !data || !data.length) return;
        var s = setupCanvas(canvas);
        var ctx = s.ctx, w = s.w, h = s.h;
        var pad = { top: 40, right: 30, bottom: 40, left: 55 };
        var cw = w - pad.left - pad.right;
        var ch = h - pad.top - pad.bottom;

        ctx.fillStyle = BG_COLOR;
        ctx.fillRect(0, 0, w, h);

        var yMin = 1.0, yMax = 2.0;
        var yRange = yMax - yMin;

        for (var v = yMin; v <= yMax + 0.001; v = Math.round((v + 0.1) * 10) / 10) {
            var gy = pad.top + ch - ((v - yMin) / yRange) * ch;
            ctx.strokeStyle = GRID_COLOR;
            ctx.lineWidth = 0.5;
            ctx.beginPath();
            ctx.moveTo(pad.left, gy);
            ctx.lineTo(pad.left + cw, gy);
            ctx.stroke();

            ctx.fillStyle = TEXT_COLOR;
            ctx.font = '11px Arial';
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            ctx.fillText(v.toFixed(1), pad.left - 8, gy);
        }

        for (var i = 0; i < data.length; i++) {
            var x = pad.left + (i / (data.length - 1)) * cw;
            if (i % Math.max(1, Math.floor(data.length / 8)) === 0 || i === data.length - 1) {
                ctx.strokeStyle = GRID_COLOR;
                ctx.lineWidth = 0.5;
                ctx.beginPath();
                ctx.moveTo(x, pad.top);
                ctx.lineTo(x, pad.top + ch);
                ctx.stroke();

                ctx.fillStyle = TEXT_COLOR;
                ctx.font = '10px Arial';
                ctx.textAlign = 'center';
                ctx.textBaseline = 'top';
                ctx.fillText(formatTime(data[i].time), x, pad.top + ch + 6);
            }
        }

        [1.4, 1.5].forEach(function (thresh) {
            var ty = pad.top + ch - ((thresh - yMin) / yRange) * ch;
            ctx.strokeStyle = thresh === 1.4 ? YELLOW : RED;
            ctx.lineWidth = 1;
            ctx.setLineDash([6, 4]);
            ctx.beginPath();
            ctx.moveTo(pad.left, ty);
            ctx.lineTo(pad.left + cw, ty);
            ctx.stroke();
            ctx.setLineDash([]);

            ctx.fillStyle = thresh === 1.4 ? YELLOW : RED;
            ctx.font = '10px Arial';
            ctx.textAlign = 'left';
            ctx.textBaseline = 'bottom';
            ctx.fillText(thresh.toFixed(1), pad.left + cw + 4, ty);
        });

        ctx.save();
        ctx.beginPath();
        ctx.rect(pad.left, pad.top, cw, ch);
        ctx.clip();

        var grad = ctx.createLinearGradient(0, pad.top, 0, pad.top + ch);
        grad.addColorStop(0, 'rgba(0,230,118,0.25)');
        grad.addColorStop(1, 'rgba(0,230,118,0)');

        ctx.beginPath();
        for (var i = 0; i < data.length; i++) {
            var x = pad.left + (i / (data.length - 1)) * cw;
            var y = pad.top + ch - ((data[i].pue_value - yMin) / yRange) * ch;
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.lineTo(pad.left + cw, pad.top + ch);
        ctx.lineTo(pad.left, pad.top + ch);
        ctx.closePath();
        ctx.fillStyle = grad;
        ctx.fill();
        ctx.restore();

        for (var i = 1; i < data.length; i++) {
            var x0 = pad.left + ((i - 1) / (data.length - 1)) * cw;
            var y0 = pad.top + ch - ((data[i - 1].pue_value - yMin) / yRange) * ch;
            var x1 = pad.left + (i / (data.length - 1)) * cw;
            var y1 = pad.top + ch - ((data[i].pue_value - yMin) / yRange) * ch;
            var midPue = (data[i - 1].pue_value + data[i].pue_value) / 2;
            ctx.strokeStyle = pueLineColor(midPue);
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(x0, y0);
            ctx.lineTo(x1, y1);
            ctx.stroke();
        }

        var lastPue = data[data.length - 1].pue_value;
        var pueStr = lastPue.toFixed(2);
        ctx.fillStyle = pueLineColor(lastPue);
        ctx.font = 'bold 36px Arial';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'top';
        ctx.fillText(pueStr, w - pad.right - 10, pad.top + 8);
        ctx.font = '14px Arial';
        ctx.fillText('PUE', w - pad.right - 10, pad.top + 48);

        var chartData = data;
        var padRef = pad, cwRef = cw, chRef = ch, yMinRef = yMin, yRangeRef = yRange;

        canvas._pueHover = function (e) {
            var rect = canvas.getBoundingClientRect();
            var mx = e.clientX - rect.left;
            var my = e.clientY - rect.top;

            var dpr = window.devicePixelRatio || 1;
            var cCtx = canvas.getContext('2d');
            cCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
            cCtx.fillStyle = BG_COLOR;
            cCtx.fillRect(0, 0, w, h);

            Charts.prototype.drawPUETrend(canvasId, chartData);

            if (mx < padRef.left || mx > padRef.left + cwRef || my < padRef.top || my > padRef.top + chRef) return;

            cCtx.strokeStyle = 'rgba(144,164,174,0.5)';
            cCtx.lineWidth = 1;
            cCtx.setLineDash([4, 4]);
            cCtx.beginPath();
            cCtx.moveTo(mx, padRef.top);
            cCtx.lineTo(mx, padRef.top + chRef);
            cCtx.stroke();
            cCtx.beginPath();
            cCtx.moveTo(padRef.left, my);
            cCtx.lineTo(padRef.left + cwRef, my);
            cCtx.stroke();
            cCtx.setLineDash([]);

            var idx = Math.round(((mx - padRef.left) / cwRef) * (chartData.length - 1));
            idx = Math.max(0, Math.min(chartData.length - 1, idx));
            var pt = chartData[idx];
            var px = padRef.left + (idx / (chartData.length - 1)) * cwRef;
            var py = padRef.top + chRef - ((pt.pue_value - yMinRef) / yRangeRef) * chRef;

            cCtx.fillStyle = pueLineColor(pt.pue_value);
            cCtx.beginPath();
            cCtx.arc(px, py, 5, 0, Math.PI * 2);
            cCtx.fill();

            var tipLines = [
                formatTime(pt.time),
                'PUE: ' + pt.pue_value.toFixed(2),
                'IT: ' + (pt.it_power != null ? pt.it_power.toFixed(1) + ' kW' : '-'),
                'Cooling: ' + (pt.cooling_power != null ? pt.cooling_power.toFixed(1) + ' kW' : '-'),
                'Dist.Loss: ' + (pt.distribution_loss != null ? pt.distribution_loss.toFixed(1) + ' kW' : '-')
            ];
            var tipW = 150, tipH = tipLines.length * 18 + 10;
            var tx = px + 10, ty = py - tipH / 2;
            if (tx + tipW > w - padRef.right) tx = px - tipW - 10;
            if (ty < padRef.top) ty = padRef.top;
            if (ty + tipH > padRef.top + chRef) ty = padRef.top + chRef - tipH;

            cCtx.fillStyle = 'rgba(22,32,64,0.92)';
            cCtx.strokeStyle = GRID_COLOR;
            cCtx.lineWidth = 1;
            cCtx.beginPath();
            cCtx.roundRect(tx, ty, tipW, tipH, 4);
            cCtx.fill();
            cCtx.stroke();

            cCtx.fillStyle = TEXT_COLOR;
            cCtx.font = '11px Arial';
            cCtx.textAlign = 'left';
            cCtx.textBaseline = 'top';
            tipLines.forEach(function (line, li) {
                cCtx.fillText(line, tx + 8, ty + 6 + li * 18);
            });
        };

        canvas.removeEventListener('mousemove', canvas._pueHoverRef);
        canvas._pueHoverRef = canvas._pueHover.bind(canvas);
        canvas.addEventListener('mousemove', canvas._pueHoverRef);
    };

    Charts.prototype.drawSankey = function (canvasId, data) {
        var canvas = document.getElementById(canvasId);
        if (!canvas || !data) return;
        var s = setupCanvas(canvas);
        var ctx = s.ctx, w = s.w, h = s.h;

        ctx.fillStyle = BG_COLOR;
        ctx.fillRect(0, 0, w, h);

        var nodes = data.nodes || [];
        var links = data.links || [];
        if (!nodes.length) return;

        var pad = { top: 40, right: 100, bottom: 40, left: 100 };
        var cw = w - pad.left - pad.right;
        var ch = h - pad.top - pad.bottom;
        var nodeW = 20;
        var colCount = 3;
        var colSpacing = cw / (colCount - 1);

        var columns = [[], [], []];
        nodes.forEach(function (n) {
            if (n.column != null) columns[n.column].push(n);
        });
        if (columns[0].length === 0 && columns[1].length === 0 && columns[2].length === 0) {
            var zoneNodes = nodes.filter(function (n) { return n.name !== '总冷量供给'; });
            columns[0] = nodes.filter(function (n) { return n.name === '总冷量供给'; });
            columns[1] = zoneNodes.filter(function (n, i) { return i % 2 === 0; });
            columns[2] = zoneNodes.filter(function (n, i) { return i % 2 === 1; });
        }

        var nodePositions = {};
        var totalVal = function (col) { return col.reduce(function (s, n) { return s + n.value; }, 0); };

        columns.forEach(function (col, ci) {
            var cx = pad.left + ci * colSpacing;
            var tv = totalVal(col) || 1;
            var yOff = pad.top;
            col.forEach(function (n) {
                var nh = Math.max(8, (n.value / tv) * ch);
                nodePositions[n.name] = {
                    x: cx,
                    y: yOff,
                    w: nodeW,
                    h: nh,
                    color: n.color || GREEN
                };
                yOff += nh + 4;
            });
        });

        links.forEach(function (link) {
            var src = nodePositions[link.source];
            var tgt = nodePositions[link.target];
            if (!src || !tgt) return;

            var sx = src.x + src.w;
            var sy1 = src.y;
            var sy2 = src.y + src.h;
            var tx = tgt.x;
            var ty1 = tgt.y;
            var ty2 = tgt.y + tgt.h;
            var cpOff = (tx - sx) * 0.5;

            var srcTotal = 0;
            links.filter(function (l) { return l.source === link.source; }).forEach(function (l) { srcTotal += l.value; });
            var srcRatio = link.value / (srcTotal || 1);

            var tgtTotal = 0;
            links.filter(function (l) { return l.target === link.target; }).forEach(function (l) { tgtTotal += l.value; });
            var tgtRatio = link.value / (tgtTotal || 1);

            var linkH = src.h * srcRatio;
            var tgtLinkH = tgt.h * tgtRatio;

            var linkSy = src.y;
            links.filter(function (l) { return l.source === link.source && l !== link; }).forEach(function (l) {
                if (nodes.findIndex(function (n) { return n.name === l.source; }) < nodes.findIndex(function (n) { return n.name === link.source; })) {
                    linkSy += src.h * (l.value / (srcTotal || 1));
                }
            });

            ctx.fillStyle = src.color ? hexToRgba(src.color, 0.35) : 'rgba(0,230,118,0.35)';
            ctx.beginPath();
            ctx.moveTo(sx, linkSy);
            ctx.bezierCurveTo(sx + cpOff, linkSy, tx - cpOff, tgt.y, tx, tgt.y);
            ctx.lineTo(tx, tgt.y + tgtLinkH);
            ctx.bezierCurveTo(tx - cpOff, tgt.y + tgtLinkH, sx + cpOff, linkSy + linkH, sx, linkSy + linkH);
            ctx.closePath();
            ctx.fill();
        });

        Object.keys(nodePositions).forEach(function (name) {
            var np = nodePositions[name];
            ctx.fillStyle = np.color;
            ctx.fillRect(np.x, np.y, np.w, np.h);

            ctx.fillStyle = TEXT_COLOR;
            ctx.font = '11px Arial';
            ctx.textAlign = 'left';
            ctx.textBaseline = 'middle';
            ctx.fillText(name, np.x + np.w + 6, np.y + np.h / 2 - 7);
            ctx.fillStyle = '#e0e0e0';
            ctx.font = '10px Arial';
            ctx.fillText(np.value.toFixed ? np.value.toFixed(0) + ' kW' : np.value + ' kW', np.x + np.w + 6, np.y + np.h / 2 + 7);
        });
    };

    function hexToRgba(hex, alpha) {
        if (hex.charAt(0) === '#') hex = hex.slice(1);
        if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
        var r = parseInt(hex.substring(0, 2), 16);
        var g = parseInt(hex.substring(2, 4), 16);
        var b = parseInt(hex.substring(4, 6), 16);
        return 'rgba(' + r + ',' + g + ',' + b + ',' + alpha + ')';
    }

    Charts.prototype.drawDeviceTrend = function (canvasId, data) {
        var canvas = document.getElementById(canvasId);
        if (!canvas || !data || !data.length) return;
        var s = setupCanvas(canvas);
        var ctx = s.ctx, w = s.w, h = s.h;

        var pad = { top: 50, right: 65, bottom: 40, left: 55 };
        var cw = w - pad.left - pad.right;
        var ch = h - pad.top - pad.bottom;

        ctx.fillStyle = BG_COLOR;
        ctx.fillRect(0, 0, w, h);

        var tempMin = 0, tempMax = 40;
        var powerMin = 0, powerMax = 500;
        var copMin = 0, copMax = 10;

        function tempY(v) { return pad.top + ch - ((v - tempMin) / (tempMax - tempMin)) * ch; }
        function powerY(v) { return pad.top + ch - ((v - powerMin) / (powerMax - powerMin)) * ch; }
        function copY(v) { return pad.top + ch - ((v - copMin) / (copMax - copMin)) * ch; }

        for (var t = tempMin; t <= tempMax; t += 10) {
            var y = tempY(t);
            ctx.strokeStyle = GRID_COLOR;
            ctx.lineWidth = 0.5;
            ctx.beginPath();
            ctx.moveTo(pad.left, y);
            ctx.lineTo(pad.left + cw, y);
            ctx.stroke();
            ctx.fillStyle = TEXT_COLOR;
            ctx.font = '10px Arial';
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            ctx.fillText(t + '°C', pad.left - 6, y);
        }

        for (var p = powerMin; p <= powerMax; p += 100) {
            var y = powerY(p);
            if (y < pad.top || y > pad.top + ch) continue;
            ctx.fillStyle = YELLOW;
            ctx.font = '10px Arial';
            ctx.textAlign = 'left';
            ctx.textBaseline = 'middle';
            ctx.fillText(p + 'kW', pad.left + cw + 6, y);
        }

        for (var i = 0; i < data.length; i++) {
            if (i % Math.max(1, Math.floor(data.length / 8)) === 0 || i === data.length - 1) {
                var x = pad.left + (i / (data.length - 1)) * cw;
                ctx.strokeStyle = GRID_COLOR;
                ctx.lineWidth = 0.5;
                ctx.beginPath();
                ctx.moveTo(x, pad.top);
                ctx.lineTo(x, pad.top + ch);
                ctx.stroke();
                ctx.fillStyle = TEXT_COLOR;
                ctx.font = '10px Arial';
                ctx.textAlign = 'center';
                ctx.textBaseline = 'top';
                ctx.fillText(formatTime(data[i].time), x, pad.top + ch + 6);
            }
        }

        var metrics = [
            { key: 'supply_temp', color: BLUE, label: '供水温度', yFn: tempY },
            { key: 'return_temp', color: ORANGE, label: '回水温度', yFn: tempY },
            { key: 'flow_rate', color: CYAN, label: '流量', yFn: powerY },
            { key: 'power', color: YELLOW, label: '功率', yFn: powerY },
            { key: 'cop', color: GREEN, label: 'COP', yFn: copY }
        ];

        var legendX = pad.left;
        metrics.forEach(function (m, mi) {
            ctx.fillStyle = m.color;
            ctx.fillRect(legendX + mi * 90, 10, 14, 3);
            ctx.fillStyle = TEXT_COLOR;
            ctx.font = '11px Arial';
            ctx.textAlign = 'left';
            ctx.textBaseline = 'middle';
            ctx.fillText(m.label, legendX + mi * 90 + 18, 14);
        });

        ctx.save();
        ctx.beginPath();
        ctx.rect(pad.left, pad.top, cw, ch);
        ctx.clip();

        metrics.forEach(function (m) {
            ctx.strokeStyle = m.color;
            ctx.lineWidth = 1.5;
            ctx.beginPath();
            for (var i = 0; i < data.length; i++) {
                var x = pad.left + (i / (data.length - 1)) * cw;
                var val = data[i][m.key];
                if (val == null) continue;
                var y = m.yFn(val);
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            }
            ctx.stroke();
        });
        ctx.restore();

        var chartData = data;
        var padRef = pad, cwRef = cw, chRef = ch;

        canvas._devHover = function (e) {
            var rect = canvas.getBoundingClientRect();
            var mx = e.clientX - rect.left;
            var my = e.clientY - rect.top;

            if (mx < padRef.left || mx > padRef.left + cwRef || my < padRef.top || my > padRef.top + chRef) {
                var dpr = window.devicePixelRatio || 1;
                var cCtx = canvas.getContext('2d');
                cCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
                Charts.prototype.drawDeviceTrend(canvasId, chartData);
                return;
            }

            var dpr = window.devicePixelRatio || 1;
            var cCtx = canvas.getContext('2d');
            cCtx.setTransform(dpr, 0, 0, dpr, 0, 0);

            cCtx.strokeStyle = 'rgba(144,164,174,0.5)';
            cCtx.lineWidth = 1;
            cCtx.setLineDash([4, 4]);
            cCtx.beginPath();
            cCtx.moveTo(mx, padRef.top);
            cCtx.lineTo(mx, padRef.top + chRef);
            cCtx.stroke();
            cCtx.beginPath();
            cCtx.moveTo(padRef.left, my);
            cCtx.lineTo(padRef.left + cwRef, my);
            cCtx.stroke();
            cCtx.setLineDash([]);

            var idx = Math.round(((mx - padRef.left) / cwRef) * (chartData.length - 1));
            idx = Math.max(0, Math.min(chartData.length - 1, idx));
            var pt = chartData[idx];
            var px = padRef.left + (idx / (chartData.length - 1)) * cwRef;

            metrics.forEach(function (m) {
                var val = pt[m.key];
                if (val == null) return;
                var py = m.yFn(val);
                cCtx.fillStyle = m.color;
                cCtx.beginPath();
                cCtx.arc(px, py, 4, 0, Math.PI * 2);
                cCtx.fill();
            });

            var tipLines = [formatTime(pt.time)];
            metrics.forEach(function (m) {
                var val = pt[m.key];
                tipLines.push(m.label + ': ' + (val != null ? val.toFixed(2) : '-'));
            });
            var tipW = 150, tipH = tipLines.length * 18 + 10;
            var tx = px + 10, ty = my - tipH / 2;
            if (tx + tipW > w - padRef.right) tx = px - tipW - 10;
            if (ty < padRef.top) ty = padRef.top;
            if (ty + tipH > padRef.top + chRef) ty = padRef.top + chRef - tipH;

            cCtx.fillStyle = 'rgba(22,32,64,0.92)';
            cCtx.strokeStyle = GRID_COLOR;
            cCtx.lineWidth = 1;
            cCtx.beginPath();
            cCtx.roundRect(tx, ty, tipW, tipH, 4);
            cCtx.fill();
            cCtx.stroke();

            cCtx.fillStyle = TEXT_COLOR;
            cCtx.font = '11px Arial';
            cCtx.textAlign = 'left';
            cCtx.textBaseline = 'top';
            tipLines.forEach(function (line, li) {
                cCtx.fillText(line, tx + 8, ty + 6 + li * 18);
            });
        };

        canvas.removeEventListener('mousemove', canvas._devHoverRef);
        canvas._devHoverRef = canvas._devHover.bind(canvas);
        canvas.addEventListener('mousemove', canvas._devHoverRef);
    };

    Charts.prototype.drawRankingTable = function (containerId, data) {
        var container = document.getElementById(containerId);
        if (!container) return;
        if (!data || !data.length) {
            container.innerHTML = '';
            return;
        }

        var headers = ['排名', '设备编号', '设备名称', '设备类型', '平均COP', '平均功率(kW)', '能效等级'];
        var html = '<table style="width:100%;border-collapse:collapse;font-size:13px;">';
        html += '<thead><tr>';
        headers.forEach(function (h) {
            html += '<th style="padding:8px 6px;text-align:left;border-bottom:1px solid #1e3a5f;color:#90a4ae;font-weight:600;">' + h + '</th>';
        });
        html += '</tr></thead><tbody>';

        data.forEach(function (item, idx) {
            var bg = '';
            if (item.avg_cop >= 5) bg = 'rgba(0,230,118,0.08)';
            else if (item.avg_cop >= 3.5) bg = 'rgba(255,214,0,0.08)';
            else bg = 'rgba(255,23,68,0.08)';

            var copColor = '';
            if (item.avg_cop >= 5) copColor = GREEN;
            else if (item.avg_cop >= 3.5) copColor = YELLOW;
            else copColor = RED;

            var level = '';
            var levelBg = '';
            if (item.avg_cop >= 5) { level = '优'; levelBg = GREEN; }
            else if (item.avg_cop >= 3.5) { level = '良'; levelBg = YELLOW; }
            else { level = '差'; levelBg = RED; }

            html += '<tr style="background:' + bg + ';">';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:#e0e0e0;">' + (idx + 1) + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:#e0e0e0;">' + (item.device_code || '') + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:#e0e0e0;">' + (item.device_name || '') + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:#e0e0e0;">' + (item.device_type || '') + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:' + copColor + ';font-weight:bold;">' + (item.avg_cop != null ? item.avg_cop.toFixed(2) : '-') + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;color:#e0e0e0;">' + (item.avg_power != null ? item.avg_power.toFixed(1) : '-') + '</td>';
            html += '<td style="padding:7px 6px;border-bottom:1px solid #1e3a5f;"><span style="display:inline-block;padding:2px 10px;border-radius:10px;font-size:11px;font-weight:bold;color:#0a1628;background:' + levelBg + ';">' + level + '</span></td>';
            html += '</tr>';
        });

        html += '</tbody></table>';
        container.innerHTML = html;
    };

    Charts.prototype.drawAlarmList = function (containerId, alarms) {
        var container = document.getElementById(containerId);
        if (!container) return;
        if (!alarms || !alarms.length) {
            container.innerHTML = '<div style="text-align:center;color:#90a4ae;padding:20px;">暂无告警</div>';
            return;
        }

        var html = '';
        alarms.forEach(function (alarm) {
            var level = alarm.alarm_level || alarm.level || 1;
            var borderColor = level === 2 ? RED : YELLOW;
            var icon = level === 2 ? '🔴' : '⚠';
            var levelClass = level === 2 ? 'level2' : 'level1';

            html += '<div style="background:#162040;border-left:3px solid ' + borderColor + ';border-radius:4px;padding:10px 12px;margin-bottom:8px;">';
            html += '<div style="display:flex;align-items:center;gap:6px;margin-bottom:4px;">';
            html += '<span>' + icon + '</span>';
            html += '<span style="color:#e0e0e0;font-size:12px;font-weight:600;">' + (alarm.device_name || '') + '</span>';
            html += '</div>';
            html += '<div style="color:#90a4ae;font-size:11px;margin-bottom:4px;">' + (alarm.message || '') + '</div>';
            html += '<div style="display:flex;justify-content:space-between;align-items:center;">';
            html += '<span style="color:#546e7a;font-size:10px;">' + (alarm.time ? new Date(alarm.time).toLocaleString('zh-CN') : '') + '</span>';

            if (!alarm.acknowledged) {
                html += '<button onclick="CoolingPlatform.Charts.ackAlarm(\'' + alarm.id + '\', this)" style="background:transparent;border:1px solid #1e3a5f;color:#90a4ae;font-size:10px;padding:2px 8px;border-radius:3px;cursor:pointer;">确认</button>';
            } else {
                html += '<span style="color:#546e7a;font-size:10px;">已确认</span>';
            }

            html += '</div></div>';
        });

        container.innerHTML = html;
    };

    Charts.ackAlarm = function (alarmId, btn) {
        fetch('/api/alarms/' + alarmId + '/ack', { method: 'POST' })
            .then(function (res) {
                if (res.ok) {
                    var span = document.createElement('span');
                    span.style.color = '#546e7a';
                    span.style.fontSize = '10px';
                    span.textContent = '已确认';
                    btn.replaceWith(span);
                }
            })
            .catch(function () {});
    };

    NS.Charts = Charts;
})(window);
