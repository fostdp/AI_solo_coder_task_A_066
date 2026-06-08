(function (global) {
    var NS = global.CoolingPlatform = global.CoolingPlatform || {};

    var TAB_MAP = {
        '3d': { panel: 'tab-scene3d', btn: 'scene3d' },
        'pue': { panel: 'tab-pue-trend', btn: 'pue-trend' },
        'sankey': { panel: 'tab-sankey', btn: 'sankey' },
        'ranking': { panel: 'tab-ranking', btn: 'ranking' }
    };

    var TYPE_LABELS = {
        chiller: '冷水机组',
        cooling_tower: '冷却塔',
        precision_ac: '精密空调',
        cdu: 'CDU'
    };

    function api(url, options) {
        return fetch(url, options).then(function (res) {
            if (!res.ok) throw new Error(res.statusText);
            return res.json();
        });
    }

    function App() {
        this.ws = null;
        this.scene3d = null;
        this.currentTab = '3d';
        this.devices = [];
        this.alarms = [];
        this.suggestions = [];
        this.pueData = [];
        this.deviceStates = [];
    }

    App.prototype.init = function () {
        var self = this;

        this.scene3d = new CoolingPlatform.Scene3D();
        this.scene3d.init('scene3d-container');

        this.scene3d.onDeviceClick(function (device) {
            self.showDeviceDetail(device);
        });

        var tabBtns = document.querySelectorAll('.tab-btn');
        tabBtns.forEach(function (btn) {
            btn.addEventListener('click', function () {
                var dataTab = btn.getAttribute('data-tab');
                var tabName;
                if (dataTab === 'scene3d') tabName = '3d';
                else if (dataTab === 'pue-trend') tabName = 'pue';
                else tabName = dataTab;
                self.switchTab(tabName);
            });
        });

        document.getElementById('device-detail-close').addEventListener('click', function () {
            self.hideDeviceDetail();
        });

        this.connectWebSocket();
        this.fetchDevices();
        this.fetchPUE();
        this.fetchAlarms();
        this.fetchSuggestions();
        this.fetchDeviceStates();
        this.fetchSankey();
        this.fetchRanking();

        this.scene3d.animate();
    };

    App.prototype.connectWebSocket = function () {
        var self = this;
        var wsUrl = 'ws://' + location.host + '/ws';
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = function () {
            var statusEl = document.getElementById('ws-status');
            statusEl.querySelector('.ws-dot').style.background = '#00e676';
            statusEl.querySelector('.ws-text').textContent = '已连接';
        };

        this.ws.onmessage = function (evt) {
            var msg = JSON.parse(evt.data);
            switch (msg.type) {
                case 'telemetry':
                    self.scene3d.updateAllDevices(msg.data);
                    self.updateDeviceTree(msg.data);
                    break;
                case 'pue_update':
                    var pueRecords = Array.isArray(msg.data) ? msg.data : [msg.data];
                    if (pueRecords.length > 0) {
                        var lastRecord = pueRecords[pueRecords.length - 1];
                        self.updatePUEDisplay(lastRecord);
                        self.pueData = self.pueData.concat(pueRecords);
                    }
                    if (self.currentTab === 'pue') {
                        CoolingPlatform.Charts.prototype.drawPUETrend('pue-chart', self.pueData);
                    }
                    break;
                case 'alarm':
                    self.alarms.unshift(msg.data);
                    CoolingPlatform.Charts.prototype.drawAlarmList('alarm-list', self.alarms);
                    self.updateAlarmCounts();
                    self.showNotification(msg.data);
                    break;
                case 'optimization':
                    self.suggestions.unshift(msg.data);
                    self.renderSuggestions();
                    break;
            }
        };

        this.ws.onclose = function () {
            var statusEl = document.getElementById('ws-status');
            statusEl.querySelector('.ws-dot').style.background = '#ff1744';
            statusEl.querySelector('.ws-text').textContent = '已断开';
            setTimeout(function () {
                self.connectWebSocket();
            }, 5000);
        };

        this.ws.onerror = function (err) {
            console.error(err);
        };
    };

    App.prototype.fetchDevices = function () {
        var self = this;
        api('/api/devices').then(function (data) {
            self.devices = data;
            self.renderDeviceTree();
            self.scene3d.createDevices(data);
            self.scene3d.createPipes(data);
            self.scene3d.addFlowAnimation();
        }).catch(function () {});
    };

    App.prototype.renderDeviceTree = function () {
        var self = this;
        var tree = document.querySelector('.device-tree');
        tree.innerHTML = '';

        var groups = {};
        this.devices.forEach(function (d) {
            if (!groups[d.type]) groups[d.type] = [];
            groups[d.type].push(d);
        });

        var typeOrder = ['chiller', 'cooling_tower', 'precision_ac', 'cdu'];
        typeOrder.forEach(function (type) {
            var list = groups[type];
            if (!list) return;

            var group = document.createElement('div');
            group.className = 'device-group';

            var header = document.createElement('div');
            header.className = 'device-group-header';
            header.setAttribute('data-group', type + 's');
            header.innerHTML = '<span class="expand-icon">▶</span><span class="group-label">' + (TYPE_LABELS[type] || type) + '</span><span class="group-count">' + list.length + '</span>';
            header.addEventListener('click', function () {
                var items = group.querySelector('.device-group-items');
                var icon = header.querySelector('.expand-icon');
                if (items.style.display === 'none') {
                    items.style.display = '';
                    icon.textContent = '▶';
                } else {
                    items.style.display = 'none';
                    icon.textContent = '▶';
                }
            });

            var items = document.createElement('div');
            items.className = 'device-group-items';

            list.forEach(function (d) {
                var item = document.createElement('div');
                item.className = 'device-item';
                item.setAttribute('data-device', d.id);

                var cop = d.cop || 0;
                var status = 'good';
                if (cop < 4) status = 'error';
                else if (cop < 6) status = 'warning';

                item.innerHTML = '<span class="status-dot" data-status="' + status + '"></span><span class="device-name">' + (d.name || d.id) + '</span><span class="device-cop">COP ' + (cop ? cop.toFixed(1) : '-') + '</span>';
                item.addEventListener('click', function () {
                    self.showDeviceDetail(d);
                });

                items.appendChild(item);
            });

            group.appendChild(header);
            group.appendChild(items);
            tree.appendChild(group);
        });
    };

    App.prototype.fetchDeviceStates = function () {
        var self = this;
        api('/api/devices/states').then(function (data) {
            self.deviceStates = data;
            self.scene3d.updateAllDevices(data);
            self.updateDeviceTree(data);
        }).catch(function () {});
    };

    App.prototype.fetchPUE = function () {
        var self = this;
        api('/api/pue/current').then(function (data) {
            if (data) {
                self.updatePUEDisplay(data);
            }
        }).catch(function () {});
        api('/api/pue/trend?hours=24').then(function (data) {
            self.pueData = data;
            if (self.currentTab === 'pue') {
                CoolingPlatform.Charts.prototype.drawPUETrend('pue-chart', data);
            }
        }).catch(function () {});
    };

    App.prototype.fetchAlarms = function () {
        var self = this;
        api('/api/alarms?limit=50').then(function (data) {
            self.alarms = data;
            CoolingPlatform.Charts.prototype.drawAlarmList('alarm-list', data);
        }).catch(function () {});
        this.updateAlarmCounts();
    };

    App.prototype.fetchSuggestions = function () {
        var self = this;
        api('/api/suggestions').then(function (data) {
            self.suggestions = data;
            self.renderSuggestions();
        }).catch(function () {});
    };

    App.prototype.renderSuggestions = function () {
        var panel = document.getElementById('suggestion-panel');
        if (!this.suggestions || !this.suggestions.length) {
            panel.innerHTML = '<div style="text-align:center;color:#90a4ae;padding:20px;">暂无建议</div>';
            return;
        }
        var html = '';
        this.suggestions.forEach(function (s) {
            html += '<div style="background:#162040;border-left:3px solid #00bcd4;border-radius:4px;padding:10px 12px;margin-bottom:8px;">';
            html += '<div style="color:#e0e0e0;font-size:12px;font-weight:600;margin-bottom:4px;">' + (s.suggestion_type === 'cooling_redistribution' ? '冷量分配优化' : (s.suggestion_type || '')) + '</div>';
            html += '<div style="color:#90a4ae;font-size:11px;">' + (s.reason || s.message || '') + '</div>';
            if (s.expected_saving > 0) {
                html += '<div style="color:#00e676;font-size:11px;margin-top:4px;">预计节能: ' + s.expected_saving.toFixed(1) + ' kW</div>';
            }
            if (s.zone) {
                html += '<div style="color:#546e7a;font-size:10px;margin-top:2px;">区域: ' + s.zone + '</div>';
            }
            html += '</div>';
        });
        panel.innerHTML = html;
    };

    App.prototype.fetchSankey = function () {
        var self = this;
        api('/api/sankey').then(function (data) {
            if (self.currentTab === 'sankey') {
                CoolingPlatform.Charts.prototype.drawSankey('sankey-chart', data);
            }
        }).catch(function () {});
    };

    App.prototype.fetchRanking = function () {
        var self = this;
        api('/api/ranking').then(function (data) {
            if (self.currentTab === 'ranking') {
                CoolingPlatform.Charts.prototype.drawRankingTable('ranking-table', data);
            }
        }).catch(function () {});
    };

    App.prototype.showDeviceDetail = function (device) {
        var self = this;
        var panel = document.getElementById('device-detail-panel');
        document.getElementById('device-detail-name').textContent = device.name || device.id;

        var paramsEl = document.getElementById('device-detail-params');
        var paramDefs = [
            { label: '供水温度', key: 'supply_temp', unit: '°C' },
            { label: '回水温度', key: 'return_temp', unit: '°C' },
            { label: '流量', key: 'flow_rate', unit: 'm³/h' },
            { label: '功率', key: 'power', unit: 'kW' },
            { label: 'COP', key: 'cop', unit: '' },
            { label: '压力', key: 'pressure', unit: 'kPa' }
        ];

        var paramsHtml = '';
        paramDefs.forEach(function (p) {
            var val = device[p.key];
            paramsHtml += '<div style="display:flex;justify-content:space-between;padding:4px 0;border-bottom:1px solid #1e3a5f;">';
            paramsHtml += '<span style="color:#90a4ae;font-size:12px;">' + p.label + '</span>';
            paramsHtml += '<span style="color:#e0e0e0;font-size:12px;font-weight:600;">' + (val != null ? val.toFixed(2) + ' ' + p.unit : '-') + '</span>';
            paramsHtml += '</div>';
        });
        paramsEl.innerHTML = paramsHtml;

        api('/api/telemetry?device_id=' + device.id + '&hours=24').then(function (data) {
            CoolingPlatform.Charts.prototype.drawDeviceTrend('device-trend-chart', data);
        }).catch(function () {});

        panel.classList.add('active');
    };

    App.prototype.hideDeviceDetail = function () {
        document.getElementById('device-detail-panel').classList.remove('active');
    };

    App.prototype.switchTab = function (tabName) {
        this.currentTab = tabName;

        document.querySelectorAll('.tab-panel').forEach(function (p) { p.classList.remove('active'); });
        document.querySelectorAll('.tab-btn').forEach(function (b) { b.classList.remove('active'); });

        var info = TAB_MAP[tabName];
        if (info) {
            var panel = document.getElementById(info.panel);
            if (panel) panel.classList.add('active');
            var btn = document.querySelector('.tab-btn[data-tab="' + info.btn + '"]');
            if (btn) btn.classList.add('active');
        }

        if (tabName === 'pue') {
            this.fetchPUE();
        } else if (tabName === 'sankey') {
            this.fetchSankey();
        } else if (tabName === 'ranking') {
            this.fetchRanking();
        }
    };

    App.prototype.updatePUEDisplay = function (pue) {
        var el = document.getElementById('pue-value');
        var indicator = document.getElementById('pue-indicator');
        if (typeof pue === 'object' && pue !== null) {
            var pueVal = pue.pue_value || pue.pue;
            var distLoss = pue.distribution_loss;
            el.textContent = pueVal.toFixed(2);
            var distEl = document.getElementById('pue-dist-loss');
            if (distEl && distLoss != null) {
                distEl.textContent = '配电损耗: ' + distLoss.toFixed(1) + ' kW';
            }
            pue = pueVal;
        } else {
            el.textContent = pue.toFixed(2);
        }
        el.classList.remove('pulse');
        if (pue < 1.4) {
            el.style.color = '#00e676';
            indicator.style.background = '#00e676';
        } else if (pue < 1.5) {
            el.style.color = '#ffd600';
            indicator.style.background = '#ffd600';
        } else {
            el.style.color = '#ff1744';
            indicator.style.background = '#ff1744';
            el.classList.add('pulse');
        }
    };

    App.prototype.updateDeviceTree = function (states) {
        if (!states) return;
        var tree = document.querySelector('.device-tree');
        states.forEach(function (s) {
            var deviceId = s.device_id || (s.device && s.device.id);
            var cop = s.cop || (s.telemetry && s.telemetry.cop);
            var item = tree.querySelector('.device-item[data-device="' + deviceId + '"]');
            if (!item) return;
            var dot = item.querySelector('.status-dot');
            var copSpan = item.querySelector('.device-cop');
            if (cop != null) {
                if (cop < 4) dot.setAttribute('data-status', 'error');
                else if (cop < 6) dot.setAttribute('data-status', 'warning');
                else dot.setAttribute('data-status', 'good');
                copSpan.textContent = 'COP ' + cop.toFixed(1);
            }
        });
    };

    App.prototype.updateAlarmCounts = function () {
        api('/api/alarms/counts').then(function (data) {
            var l1 = document.getElementById('alarm-level1');
            var l2 = document.getElementById('alarm-level2');
            if (l1) l1.textContent = data.level1 || 0;
            if (l2) l2.textContent = data.level2 || 0;
        }).catch(function () {});
    };

    App.prototype.showNotification = function (alarm) {
        if (!('Notification' in window)) return;
        if (Notification.permission === 'granted') {
            new Notification('告警通知', { body: alarm.message || alarm.device_name || '新告警' });
        } else if (Notification.permission !== 'denied') {
            Notification.requestPermission();
        }
    };

    NS.App = App;
    NS.app = new App();

    document.addEventListener('DOMContentLoaded', function () {
        NS.app.init();
    });
})(window);
