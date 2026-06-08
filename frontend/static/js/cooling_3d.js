(function (global) {
    var NS = global.CoolingPlatform = global.CoolingPlatform || {};

    function createTextSprite(text, color) {
        var canvas = document.createElement('canvas');
        canvas.width = 256;
        canvas.height = 64;
        var ctx = canvas.getContext('2d');
        ctx.fillStyle = color || '#ffffff';
        ctx.font = 'bold 24px Arial';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(text, 128, 32);
        var texture = new THREE.CanvasTexture(canvas);
        var material = new THREE.SpriteMaterial({ map: texture, transparent: true });
        var sprite = new THREE.Sprite(material);
        sprite.scale.set(3, 0.75, 1);
        return sprite;
    }

    function copToColor(cop) {
        if (cop > 6) return 0x00e676;
        if (cop >= 4) return 0xffd600;
        return 0xff1744;
    }

    var DEVICE_SIZES = {
        chiller: [1.5, 1.5, 1.5],
        cooling_tower: [1.2, 2, 1.2],
        precision_ac: [0.8, 0.8, 0.8],
        cdu: [0.6, 1, 0.6]
    };

    var CHILLER_POSITIONS = [];
    for (var i = 0; i < 8; i++) {
        CHILLER_POSITIONS.push({ x: -12 + i * 2.5, z: -8 });
    }

    var COOLING_TOWER_POSITIONS = [];
    for (var i = 0; i < 12; i++) {
        COOLING_TOWER_POSITIONS.push({ x: -15 + i * 2.2, z: -16 });
    }

    var PRECISION_AC_POSITIONS = [];
    for (var g = 0; g < 8; g++) {
        var gx = -12 + g * 2.5;
        for (var j = 0; j < 10; j++) {
            PRECISION_AC_POSITIONS.push({ x: gx - 0.9 + j * 0.2, z: 4 });
        }
    }

    var CDU_GROUP_X = [-8, -4, 0, 4];
    var CDU_POSITIONS = [];
    for (var g = 0; g < 4; g++) {
        for (var j = 0; j < 5; j++) {
            CDU_POSITIONS.push({ x: CDU_GROUP_X[g] - 0.6 + j * 0.3, z: 10 });
        }
    }

    var POSITION_MAP = {
        chiller: CHILLER_POSITIONS,
        cooling_tower: COOLING_TOWER_POSITIONS,
        precision_ac: PRECISION_AC_POSITIONS,
        cdu: CDU_POSITIONS
    };

    function Scene3D(containerId) {
        this.containerId = containerId;
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.controls = null;
        this.raycaster = new THREE.Raycaster();
        this.mouse = new THREE.Vector2();
        this.deviceMeshes = {};
        this.pipeLines = [];
        this.flowSystems = [];
        this.clickCallback = null;
        this.animationId = null;
        this.clock = new THREE.Clock();
        this.init(containerId);
    }

    Scene3D.prototype.init = function (containerId) {
        var container = document.getElementById(containerId);
        this.container = container;

        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x0a0e1a);
        this.scene.fog = new THREE.Fog(0x0a0e1a, 50, 120);

        var aspect = container.clientWidth / container.clientHeight;
        this.camera = new THREE.PerspectiveCamera(60, aspect, 0.1, 1000);
        this.camera.position.set(0, 25, 35);
        this.camera.lookAt(0, 0, 0);

        this.renderer = new THREE.WebGLRenderer({ antialias: true });
        this.renderer.setSize(container.clientWidth, container.clientHeight);
        this.renderer.setPixelRatio(window.devicePixelRatio);
        container.appendChild(this.renderer.domElement);

        this.controls = new THREE.OrbitControls(this.camera, this.renderer.domElement);
        this.controls.enableDamping = true;
        this.controls.dampingFactor = 0.1;

        this.scene.add(new THREE.AmbientLight(0x334466, 0.5));

        var dirLight = new THREE.DirectionalLight(0xffffff, 0.8);
        dirLight.position.set(10, 20, 10);
        this.scene.add(dirLight);

        var pointLight = new THREE.PointLight(0x4488ff, 0.5, 30);
        pointLight.position.set(-3, 5, -8);
        this.scene.add(pointLight);

        this.scene.add(new THREE.GridHelper(60, 60, 0x1a2a4a, 0x111827));

        var self = this;
        this.renderer.domElement.addEventListener('click', function (e) {
            var rect = self.renderer.domElement.getBoundingClientRect();
            self.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
            self.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
            self.raycaster.setFromCamera(self.mouse, self.camera);
            var meshList = [];
            for (var id in self.deviceMeshes) {
                if (self.deviceMeshes.hasOwnProperty(id)) {
                    meshList.push(self.deviceMeshes[id]);
                }
            }
            var intersects = self.raycaster.intersectObjects(meshList);
            if (intersects.length > 0 && self.clickCallback) {
                self.clickCallback(intersects[0].object.userData);
            }
        });

        window.addEventListener('resize', function () {
            self.onResize();
        });
    };

    Scene3D.prototype.createDevices = function (devices) {
        var self = this;
        var posIdx = { chiller: 0, cooling_tower: 0, precision_ac: 0, cdu: 0 };

        devices.forEach(function (device) {
            var size = DEVICE_SIZES[device.type] || [1, 1, 1];
            var positions = POSITION_MAP[device.type];
            if (!positions) return;
            var idx = posIdx[device.type]++;
            var pos = positions[idx % positions.length];

            var cop = device.cop || 5;
            var color = copToColor(cop);

            var geometry = new THREE.BoxGeometry(size[0], size[1], size[2]);
            var material = new THREE.MeshPhongMaterial({
                color: color,
                transparent: true,
                opacity: 0.85,
                shininess: 80
            });
            var mesh = new THREE.Mesh(geometry, material);
            mesh.position.set(pos.x, size[1] / 2, pos.z);
            mesh.userData = device;

            var label = createTextSprite(device.name || device.type, '#ffffff');
            label.position.set(0, size[1] / 2 + 0.8, 0);
            mesh.add(label);

            self.scene.add(mesh);
            self.deviceMeshes[device.id] = mesh;
        });
    };

    Scene3D.prototype.createPipes = function (devices) {
        var self = this;

        for (var i = 0; i < CHILLER_POSITIONS.length; i++) {
            var cp = CHILLER_POSITIONS[i];
            var nearest = null;
            var minDist = Infinity;
            for (var j = 0; j < COOLING_TOWER_POSITIONS.length; j++) {
                var tp = COOLING_TOWER_POSITIONS[j];
                var dist = Math.abs(cp.x - tp.x);
                if (dist < minDist) {
                    minDist = dist;
                    nearest = tp;
                }
            }

            var supplyGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(cp.x, 0.5, cp.z),
                new THREE.Vector3(nearest.x, 0.5, nearest.z)
            ]);
            var supplyLine = new THREE.Line(supplyGeom, new THREE.LineBasicMaterial({ color: 0x2196f3, linewidth: 2 }));
            self.scene.add(supplyLine);
            self.pipeLines.push({ line: supplyLine, start: { x: cp.x, z: cp.z }, end: { x: nearest.x, z: nearest.z }, y: 0.5, type: 'chiller_to_tower_supply' });

            var returnGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(cp.x, 1.0, cp.z),
                new THREE.Vector3(nearest.x, 1.0, nearest.z)
            ]);
            var returnMat = new THREE.LineDashedMaterial({ color: 0xff5252, linewidth: 2, dashSize: 0.5, gapSize: 0.3 });
            var returnLine = new THREE.Line(returnGeom, returnMat);
            returnLine.computeLineDistances();
            self.scene.add(returnLine);
            self.pipeLines.push({ line: returnLine, start: { x: cp.x, z: cp.z }, end: { x: nearest.x, z: nearest.z }, y: 1.0, type: 'chiller_to_tower_return' });
        }

        for (var i = 0; i < CHILLER_POSITIONS.length; i++) {
            var cp = CHILLER_POSITIONS[i];
            var pacX = cp.x;

            var supplyGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(cp.x, 0.5, cp.z),
                new THREE.Vector3(pacX, 0.5, 4)
            ]);
            var supplyLine = new THREE.Line(supplyGeom, new THREE.LineBasicMaterial({ color: 0x2196f3, linewidth: 2 }));
            self.scene.add(supplyLine);
            self.pipeLines.push({ line: supplyLine, start: { x: cp.x, z: cp.z }, end: { x: pacX, z: 4 }, y: 0.5, type: 'chiller_to_ac_supply' });

            var returnGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(cp.x, 1.0, cp.z),
                new THREE.Vector3(pacX, 1.0, 4)
            ]);
            var returnMat = new THREE.LineDashedMaterial({ color: 0xff5252, linewidth: 2, dashSize: 0.5, gapSize: 0.3 });
            var returnLine = new THREE.Line(returnGeom, returnMat);
            returnLine.computeLineDistances();
            self.scene.add(returnLine);
            self.pipeLines.push({ line: returnLine, start: { x: cp.x, z: cp.z }, end: { x: pacX, z: 4 }, y: 1.0, type: 'chiller_to_ac_return' });
        }

        for (var g = 0; g < 8; g++) {
            var acX = CHILLER_POSITIONS[g].x;
            var cduGroupIdx = Math.floor(g / 2);
            var cduX = CDU_GROUP_X[cduGroupIdx];

            var supplyGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(acX, 0.5, 4),
                new THREE.Vector3(cduX, 0.5, 10)
            ]);
            var supplyLine = new THREE.Line(supplyGeom, new THREE.LineBasicMaterial({ color: 0x2196f3, linewidth: 2 }));
            self.scene.add(supplyLine);
            self.pipeLines.push({ line: supplyLine, start: { x: acX, z: 4 }, end: { x: cduX, z: 10 }, y: 0.5, type: 'ac_to_cdu_supply' });

            var returnGeom = new THREE.BufferGeometry().setFromPoints([
                new THREE.Vector3(acX, 1.0, 4),
                new THREE.Vector3(cduX, 1.0, 10)
            ]);
            var returnMat = new THREE.LineDashedMaterial({ color: 0xff5252, linewidth: 2, dashSize: 0.5, gapSize: 0.3 });
            var returnLine = new THREE.Line(returnGeom, returnMat);
            returnLine.computeLineDistances();
            self.scene.add(returnLine);
            self.pipeLines.push({ line: returnLine, start: { x: acX, z: 4 }, end: { x: cduX, z: 10 }, y: 1.0, type: 'ac_to_cdu_return' });
        }
    };

    Scene3D.prototype.updateDeviceColor = function (deviceId, cop) {
        var mesh = this.deviceMeshes[deviceId];
        if (mesh) {
            mesh.material.color.setHex(copToColor(cop));
        }
    };

    Scene3D.prototype.onDeviceClick = function (callback) {
        this.clickCallback = callback;
    };

    Scene3D.prototype.updateAllDevices = function (states) {
        var self = this;
        states.forEach(function (state) {
            var deviceId = state.device_id || (state.device && state.device.id);
            var cop = state.cop || (state.telemetry && state.telemetry.cop);
            if (deviceId !== undefined && cop !== undefined) {
                self.updateDeviceColor(deviceId, cop);
            }
        });
    };

    Scene3D.prototype.animate = function () {
        var self = this;
        function loop() {
            self.animationId = requestAnimationFrame(loop);
            self.controls.update();
            self.updateFlowParticles();
            self.renderer.render(self.scene, self.camera);
        }
        loop();
    };

    Scene3D.prototype.onResize = function () {
        var w = this.container.clientWidth;
        var h = this.container.clientHeight;
        this.camera.aspect = w / h;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(w, h);
    };

    Scene3D.prototype.addFlowAnimation = function () {
        var self = this;
        var particleTexture = this.createParticleTexture();

        this.pipeLines.forEach(function (pipe) {
            var isSupply = pipe.type.indexOf('supply') !== -1;
            var particleCount = 8;
            var positions = new Float32Array(particleCount * 3);
            var offsets = [];

            for (var i = 0; i < particleCount; i++) {
                offsets.push(i / particleCount);
                positions[i * 3] = pipe.start.x;
                positions[i * 3 + 1] = pipe.y;
                positions[i * 3 + 2] = pipe.start.z;
            }

            var geometry = new THREE.BufferGeometry();
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));

            var material = new THREE.PointsMaterial({
                color: isSupply ? 0x64b5f6 : 0xef9a9a,
                size: 0.3,
                map: particleTexture,
                transparent: true,
                opacity: 0.8,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });

            var points = new THREE.Points(geometry, material);
            self.scene.add(points);

            self.flowSystems.push({
                points: points,
                startX: pipe.start.x,
                startZ: pipe.start.z,
                endX: pipe.end.x,
                endZ: pipe.end.z,
                y: pipe.y,
                offsets: offsets,
                speed: 0.3
            });
        });
    };

    Scene3D.prototype.updateFlowParticles = function () {
        var delta = this.clock.getDelta();
        this.flowSystems.forEach(function (sys) {
            var positions = sys.points.geometry.attributes.position.array;
            for (var i = 0; i < sys.offsets.length; i++) {
                sys.offsets[i] += delta * sys.speed;
                if (sys.offsets[i] > 1) sys.offsets[i] -= 1;
                var t = sys.offsets[i];
                positions[i * 3] = sys.startX + (sys.endX - sys.startX) * t;
                positions[i * 3 + 1] = sys.y;
                positions[i * 3 + 2] = sys.startZ + (sys.endZ - sys.startZ) * t;
            }
            sys.points.geometry.attributes.position.needsUpdate = true;
        });
    };

    Scene3D.prototype.createParticleTexture = function () {
        var canvas = document.createElement('canvas');
        canvas.width = 32;
        canvas.height = 32;
        var ctx = canvas.getContext('2d');
        var gradient = ctx.createRadialGradient(16, 16, 0, 16, 16, 16);
        gradient.addColorStop(0, 'rgba(255,255,255,1)');
        gradient.addColorStop(0.3, 'rgba(255,255,255,0.8)');
        gradient.addColorStop(1, 'rgba(255,255,255,0)');
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, 32, 32);
        return new THREE.CanvasTexture(canvas);
    };

    NS.Cooling3D = Scene3D;
})(window);