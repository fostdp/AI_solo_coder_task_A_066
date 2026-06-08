import struct
import socket
import threading
import time
import random
import math

class ModbusDevice:
    def __init__(self, slave_id, device_type, device_name):
        self.slave_id = slave_id
        self.device_type = device_type
        self.device_name = device_name
        self.registers = [0] * 20
        self.running = True
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

    def simulate(self):
        cycle = 0
        while self.running:
            cycle += 1
            t = cycle * 0.05
            noise = lambda base, amp: base + amp * math.sin(t + self.slave_id * 0.7) + random.uniform(-amp * 0.3, amp * 0.3)

            if self.device_type == 'chiller':
                self.registers[0] = int(noise(7.2, 0.5) * 10)
                self.registers[1] = int(noise(12.5, 0.8) * 10)
                self.registers[2] = int(noise(180.0, 10.0) * 10)
                self.registers[3] = int(noise(850.0, 50.0) * 10)
                self.registers[4] = int(noise(0.8, 0.05) * 10)
                cop = noise(6.2, 0.8)
                if random.random() < 0.02:
                    cop = noise(3.5, 0.3)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(4200.0, 200.0) * 10)
                self.registers[7] = int(noise(8.0, 0.5) * 10)
            elif self.device_type == 'cooling_tower':
                self.registers[0] = int(noise(28.0, 2.0) * 10)
                self.registers[1] = int(noise(35.0, 2.5) * 10)
                self.registers[2] = int(noise(250.0, 20.0) * 10)
                self.registers[3] = int(noise(55.0, 5.0) * 10)
                self.registers[4] = int(noise(0.3, 0.02) * 10)
                self.registers[5] = int(noise(5.5, 0.6) * 100)
                self.registers[6] = int(noise(3500.0, 150.0) * 10)
                self.registers[7] = int(noise(30.0, 2.0) * 10)
            elif self.device_type == 'precision_ac':
                self.registers[0] = int(noise(12.0, 0.3) * 10)
                self.registers[1] = int(noise(22.0, 1.0) * 10)
                self.registers[2] = int(noise(15.0, 1.5) * 10)
                self.registers[3] = int(noise(35.0, 3.0) * 10)
                self.registers[4] = int(noise(0.6, 0.03) * 10)
                cop = noise(4.8, 0.6)
                if random.random() < 0.03:
                    cop = noise(2.8, 0.2)
                self.registers[5] = int(cop * 100)
                self.registers[6] = int(noise(150.0, 10.0) * 10)
                self.registers[7] = int(noise(24.0, 0.5) * 10)
            elif self.device_type == 'cdu':
                self.registers[0] = int(noise(18.0, 0.5) * 10)
                self.registers[1] = int(noise(35.0, 1.5) * 10)
                self.registers[2] = int(noise(8.0, 0.5) * 10)
                self.registers[3] = int(noise(22.0, 2.0) * 10)
                self.registers[4] = int(noise(0.5, 0.02) * 10)
                self.registers[5] = int(noise(5.0, 0.5) * 100)
                self.registers[6] = int(noise(200.0, 15.0) * 10)
                self.registers[7] = int(noise(20.0, 0.3) * 10)

            self.registers[8] = 1 if random.random() > 0.005 else 0
            time.sleep(30)

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
    def __init__(self, host='0.0.0.0', port=502):
        self.host = host
        self.port = port
        self.devices = {}
        self.transaction_id = 0
        self._create_devices()

    def _create_devices(self):
        for i in range(1, 9):
            dev = ModbusDevice(i, 'chiller', f'CHU-{i:03d}')
            self.devices[i] = dev
        for i in range(9, 21):
            dev = ModbusDevice(i, 'cooling_tower', f'CT-{i-8:03d}')
            self.devices[i] = dev
        for i in range(21, 101):
            dev = ModbusDevice(i, 'precision_ac', f'PAC-{i-20:03d}')
            self.devices[i] = dev
        for i in range(101, 121):
            dev = ModbusDevice(i, 'cdu', f'CDU-{i-100:03d}')
            self.devices[i] = dev

    def start(self):
        print(f"Modbus TCP Simulator starting on {self.host}:{self.port}")
        print(f"Devices: {len(self.devices)} total")
        print(f"  Chillers: slave 1-8")
        print(f"  Cooling Towers: slave 9-20")
        print(f"  Precision ACs: slave 21-100")
        print(f"  CDUs: slave 101-120")
        print(f"  Registers per device: 10 (supply_temp, return_temp, flow_rate, power, pressure, cop*100, cooling_capacity, setpoint_temp, status, reserved)")
        print(f"  Update interval: 30 seconds")

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
        except Exception as e:
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


if __name__ == '__main__':
    import argparse
    parser = argparse.ArgumentParser(description='Modbus TCP Simulator for DC Cooling Platform')
    parser.add_argument('--host', default='0.0.0.0', help='Bind address (default: 0.0.0.0)')
    parser.add_argument('--port', type=int, default=502, help='Listen port (default: 502)')
    args = parser.parse_args()

    sim = ModbusTCPSimulator(args.host, args.port)
    try:
        sim.start()
    except KeyboardInterrupt:
        print("\nSimulator stopped.")
        for dev in sim.devices.values():
            dev.running = False
