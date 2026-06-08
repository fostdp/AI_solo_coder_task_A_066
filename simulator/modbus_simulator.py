import struct
import socket
import threading
import time
import random
import math
import argparse
import json
import sys


class ModbusDevice:
    def __init__(self, slave_id, device_type, device_name, anomaly_prob=0.03, low_cop=2.5, interval=30):
        self.slave_id = slave_id
        self.device_type = device_type
        self.device_name = device_name
        self.registers = [0] * 20
        self.running = True
        self.anomaly_prob = anomaly_prob
        self.low_cop = low_cop
        self.interval = interval
        self.anomaly_active = False
        self.anomaly_end_time = 0
        self._init_registers()

    def _init_registers(self):
        if self.device_type == 'chiller':
            self.registers[0] = int(7.2 * 10)
            self.registers[1] = int(12.5 * 10)
            self.registers[2] = int(180.0 * 10)
            self.registers[3] = int(850.0 * 10)
            self.registers[4] = int(0.8 * 10)
            self.registers[5] = int(6.2 * 100)
            self.registers[6] = int(4200.0 * 10)
            self.registers[7] = int(8.0 * 10)
            self.registers[8] = 1
        elif self.device_type == 'cooling_tower':
            self.registers[0] = int(28.0 * 10)
            self.registers[1] = int(35.0 * 10)
            self.registers[2] = int(250.0 * 10)
            self.registers[3] = int(55.0 * 10)
            self.registers[4] = int(0.3 * 10)
            self.registers[5] = int(5.5 * 100)
            self.registers[6] = int(3500.0 * 10)
            self.registers[7] = int(30.0 * 10)
            self.registers[8] = 1
        elif self.device_type == 'precision_ac':
            self.registers[0] = int(12.0 * 10)
            self.registers[1] = int(22.0 * 10)
            self.registers[2] = int(15.0 * 10)
            self.registers[3] = int(35.0 * 10)
            self.registers[4] = int(0.6 * 10)
            self.registers[5] = int(4.8 * 100)
            self.registers[6] = int(150.0 * 10)
            self.registers[7] = int(24.0 * 10)
            self.registers[8] = 1
        elif self.device_type == 'cdu':
            self.registers[0] = int(18.0 * 10)
            self.registers[1] = int(35.0 * 10)
            self.registers[2] = int(8.0 * 10)
            self.registers[3] = int(22.0 * 10)
            self.registers[4] = int(0.5 * 10)
            self.registers[5] = int(5.0 * 100)
            self.registers[6] = int(200.0 * 10)
            self.registers[7] = int(20.0 * 10)
            self.registers[8] = 1

    def inject_anomaly(self, duration=300):
        self.anomaly_active = True
        self.anomaly_end_time = time.time() + duration

    def simulate(self):
        cycle = 0
        while self.running:
            cycle += 1
            t = cycle * 0.05
            noise = lambda base, amp: base + amp * math.sin(t + self.slave_id * 0.7) + random.uniform(-amp * 0.3, amp * 0.3)

            if self.anomaly_active and time.time() > self.anomaly_end_time:
                self.anomaly_active = False

            if self.device_type == 'chiller':
                self.registers[0] = int(noise(7.2, 0.5) * 10)
                self.registers[1] = int(noise(12.5, 0.8) * 10)
                self.registers[2] = int(noise(180.0, 10.0) * 10)
                self.registers[3] = int(noise(850.0, 50.0) * 10)
                self.registers[4] = int(noise(0.8, 0.05) * 10)
                cop = noise(6.2, 0.8)
                if self.anomaly_active or random.random() < self.anomaly_prob:
                    cop = noise(self.low_cop, 0.3)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(4200.0, 200.0) * 10)
                self.registers[7] = int(noise(8.0, 0.5) * 10)
            elif self.device_type == 'cooling_tower':
                self.registers[0] = int(noise(28.0, 2.0) * 10)
                self.registers[1] = int(noise(35.0, 2.5) * 10)
                self.registers[2] = int(noise(250.0, 20.0) * 10)
                self.registers[3] = int(noise(55.0, 5.0) * 10)
                self.registers[4] = int(noise(0.3, 0.02) * 10)
                cop = noise(5.5, 0.6)
                if self.anomaly_active or random.random() < self.anomaly_prob:
                    cop = noise(self.low_cop, 0.3)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(3500.0, 150.0) * 10)
                self.registers[7] = int(noise(30.0, 2.0) * 10)
            elif self.device_type == 'precision_ac':
                self.registers[0] = int(noise(12.0, 0.3) * 10)
                self.registers[1] = int(noise(22.0, 1.0) * 10)
                self.registers[2] = int(noise(15.0, 1.5) * 10)
                self.registers[3] = int(noise(35.0, 3.0) * 10)
                self.registers[4] = int(noise(0.6, 0.03) * 10)
                cop = noise(4.8, 0.6)
                if self.anomaly_active or random.random() < self.anomaly_prob:
                    cop = noise(self.low_cop, 0.2)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(150.0, 10.0) * 10)
                self.registers[7] = int(noise(24.0, 0.5) * 10)
            elif self.device_type == 'cdu':
                self.registers[0] = int(noise(18.0, 0.5) * 10)
                self.registers[1] = int(noise(35.0, 1.5) * 10)
                self.registers[2] = int(noise(8.0, 0.5) * 10)
                self.registers[3] = int(noise(22.0, 2.0) * 10)
                self.registers[4] = int(noise(0.5, 0.02) * 10)
                cop = noise(5.0, 0.5)
                if self.anomaly_active or random.random() < self.anomaly_prob:
                    cop = noise(self.low_cop, 0.3)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(200.0, 15.0) * 10)
                self.registers[7] = int(noise(20.0, 0.3) * 10)

            self.registers[8] = 1 if (not self.anomaly_active and random.random() > 0.005) else 0
            time.sleep(self.interval)

    def handle_request(self, function_code, start_addr, quantity):
        if function_code == 0x03:
            if start_addr + quantity > len(self.registers):
                return None
            values = self.registers[start_addr:start_addr + quantity]
            byte_count = quantity * 2
            data = struct.pack('B', byte_count)
            for v in values:
                data += struct.pack('>H', max(0, min(65535, int(v))))
            return data
        return None


class ModbusTCPSimulator:
    def __init__(self, host='0.0.0.0', port=502, chillers=8, towers=12, pacs=80, cdus=20,
                 anomaly_prob=0.03, low_cop=2.5, interval=30):
        self.host = host
        self.port = port
        self.devices = {}
        self.transaction_id = 0
        self.anomaly_prob = anomaly_prob
        self.low_cop = low_cop
        self.interval = interval
        self._create_devices(chillers, towers, pacs, cdus)

    def _create_devices(self, chillers, towers, pacs, cdus):
        for i in range(1, chillers + 1):
            dev = ModbusDevice(i, 'chiller', f'CHU-{i:03d}',
                               anomaly_prob=self.anomaly_prob, low_cop=self.low_cop, interval=self.interval)
            self.devices[i] = dev
        for i in range(chillers + 1, chillers + towers + 1):
            dev = ModbusDevice(i, 'cooling_tower', f'CT-{i - chillers:03d}',
                               anomaly_prob=self.anomaly_prob, low_cop=self.low_cop, interval=self.interval)
            self.devices[i] = dev
        for i in range(chillers + towers + 1, chillers + towers + pacs + 1):
            dev = ModbusDevice(i, 'precision_ac', f'PAC-{i - chillers - towers:03d}',
                               anomaly_prob=self.anomaly_prob, low_cop=self.low_cop, interval=self.interval)
            self.devices[i] = dev
        for i in range(chillers + towers + pacs + 1, chillers + towers + pacs + cdus + 1):
            dev = ModbusDevice(i, 'cdu', f'CDU-{i - chillers - towers - pacs:03d}',
                               anomaly_prob=self.anomaly_prob, low_cop=self.low_cop, interval=self.interval)
            self.devices[i] = dev

    def inject_anomaly(self, device_type=None, duration=300):
        count = 0
        for dev in self.devices.values():
            if device_type is None or dev.device_type == device_type:
                dev.inject_anomaly(duration)
                count += 1
        return count

    def start(self):
        print(f"Modbus TCP Simulator starting on {self.host}:{self.port}")
        print(f"Devices: {len(self.devices)} total")
        print(f"  Chillers: {sum(1 for d in self.devices.values() if d.device_type == 'chiller')} (slave 1-{sum(1 for d in self.devices.values() if d.device_type == 'chiller')})")
        print(f"  Cooling Towers: {sum(1 for d in self.devices.values() if d.device_type == 'cooling_tower')}")
        print(f"  Precision ACs: {sum(1 for d in self.devices.values() if d.device_type == 'precision_ac')}")
        print(f"  CDUs: {sum(1 for d in self.devices.values() if d.device_type == 'cdu')}")
        print(f"  Anomaly probability: {self.anomaly_prob} (random per cycle)")
        print(f"  Low COP threshold: {self.low_cop}")
        print(f"  Update interval: {self.interval}s")

        for dev in self.devices.values():
            t = threading.Thread(target=dev.simulate, daemon=True)
            t.start()

        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind((self.host, self.port))
        server.listen(20)

        print("Waiting for connections...")
        while True:
            try:
                client, addr = server.accept()
                print(f"Connection from {addr}")
                t = threading.Thread(target=self._handle_client, args=(client,), daemon=True)
                t.start()
            except Exception as e:
                print(f"Accept error: {e}")

    def _handle_client(self, client):
        try:
            while True:
                header = self._recv_exact(client, 7)
                if not header:
                    break
                tx_id, proto_id, length, unit_id = struct.unpack('>HHHB', header)
                pdu_len = length - 1
                pdu = self._recv_exact(client, pdu_len)
                if not pdu:
                    break
                function_code = pdu[0]

                if function_code == 0x03:
                    start_addr, quantity = struct.unpack('>HH', pdu[1:5])
                    if unit_id in self.devices:
                        response_data = self.devices[unit_id].handle_request(function_code, start_addr, quantity)
                        if response_data:
                            resp_pdu = struct.pack('B', function_code) + response_data
                            resp_len = len(resp_pdu) + 1
                            resp_header = struct.pack('>HHHB', tx_id, 0, resp_len, unit_id)
                            client.sendall(resp_header + resp_pdu)
                        else:
                            err_pdu = struct.pack('BB', function_code | 0x80, 0x02)
                            err_header = struct.pack('>HHHB', tx_id, 0, len(err_pdu) + 1, unit_id)
                            client.sendall(err_header + err_pdu)
                    else:
                        err_pdu = struct.pack('BB', function_code | 0x80, 0x01)
                        err_header = struct.pack('>HHHB', tx_id, 0, len(err_pdu) + 1, unit_id)
                        client.sendall(err_header + err_pdu)
                else:
                    err_pdu = struct.pack('BB', function_code | 0x80, 0x01)
                    err_header = struct.pack('>HHHB', tx_id, 0, len(err_pdu) + 1, unit_id)
                    client.sendall(err_header + err_pdu)
        except Exception:
            pass
        finally:
            client.close()

    def _recv_exact(self, sock, n):
        data = b''
        while len(data) < n:
            chunk = sock.recv(n - len(data))
            if not chunk:
                return None
            data += chunk
        return data


class ControlServer:
    def __init__(self, simulator, host='0.0.0.0', port=8081):
        self.simulator = simulator
        self.host = host
        self.port = port

    def start(self):
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind((self.host, self.port))
        server.listen(5)
        print(f"Control server listening on {self.host}:{self.port}")
        print(f"  Send JSON commands: {{\"action\":\"inject_anomaly\",\"device_type\":\"chiller\",\"duration\":300}}")

        while True:
            try:
                client, addr = server.accept()
                t = threading.Thread(target=self._handle, args=(client,), daemon=True)
                t.start()
            except Exception:
                break

    def _handle(self, client):
        try:
            data = b''
            while True:
                chunk = client.recv(4096)
                if not chunk:
                    break
                data += chunk
                if b'\n' in data:
                    break

            cmd = json.loads(data.decode().strip())
            action = cmd.get('action', '')

            if action == 'inject_anomaly':
                device_type = cmd.get('device_type')
                duration = cmd.get('duration', 300)
                count = self.simulator.inject_anomaly(device_type=device_type, duration=duration)
                resp = json.dumps({"status": "ok", "affected_devices": count, "duration": duration})
                client.sendall((resp + '\n').encode())
            elif action == 'status':
                resp = json.dumps({"status": "ok", "total_devices": len(self.simulator.devices),
                                   "anomaly_prob": self.simulator.anomaly_prob,
                                   "low_cop": self.simulator.low_cop,
                                   "interval": self.simulator.interval})
                client.sendall((resp + '\n').encode())
            else:
                resp = json.dumps({"status": "error", "message": f"unknown action: {action}"})
                client.sendall((resp + '\n').encode())
        except Exception as e:
            try:
                client.sendall((json.dumps({"status": "error", "message": str(e)}) + '\n').encode())
            except Exception:
                pass
        finally:
            client.close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Modbus TCP Simulator for DC Cooling Platform')
    parser.add_argument('--host', default='0.0.0.0', help='Bind address (default: 0.0.0.0)')
    parser.add_argument('--port', type=int, default=502, help='Modbus listen port (default: 502)')
    parser.add_argument('--chillers', type=int, default=8, help='Number of centrifugal chillers (default: 8)')
    parser.add_argument('--towers', type=int, default=12, help='Number of cooling towers (default: 12)')
    parser.add_argument('--pacs', type=int, default=80, help='Number of precision air conditioners (default: 80)')
    parser.add_argument('--cdus', type=int, default=20, help='Number of liquid cooling CDUs (default: 20)')
    parser.add_argument('--interval', type=int, default=30, help='Data update interval in seconds (default: 30)')
    parser.add_argument('--anomaly-prob', type=float, default=0.03,
                        help='Per-cycle probability of random COP anomaly 0.0-1.0 (default: 0.03)')
    parser.add_argument('--low-cop', type=float, default=2.5,
                        help='COP value used when anomaly is triggered (default: 2.5)')
    parser.add_argument('--control-port', type=int, default=8081,
                        help='Control server port for anomaly injection (default: 8081)')
    args = parser.parse_args()

    sim = ModbusTCPSimulator(
        host=args.host, port=args.port,
        chillers=args.chillers, towers=args.towers,
        pacs=args.pacs, cdus=args.cdus,
        anomaly_prob=args.anomaly_prob, low_cop=args.low_cop,
        interval=args.interval
    )

    ctrl = ControlServer(sim, host=args.host, port=args.control_port)
    threading.Thread(target=ctrl.start, daemon=True).start()

    try:
        sim.start()
    except KeyboardInterrupt:
        print("\nSimulator stopped.")
        for dev in sim.devices.values():
            dev.running = False
