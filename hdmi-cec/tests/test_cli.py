import importlib.util
from pathlib import Path
import struct
import tempfile
from types import SimpleNamespace
import unittest

spec=importlib.util.spec_from_file_location("cec",Path(__file__).resolve().parents[1]/"cec.py")
cec=importlib.util.module_from_spec(spec)
spec.loader.exec_module(cec)

class FlashValidation(unittest.TestCase):
    def test_board_selection(self):
        a=SimpleNamespace(idProduct=15,bus=1,address=2)
        b=SimpleNamespace(idProduct=15,bus=1,address=3)
        self.assertIs(cec.choose_device([a]),a)
        self.assertIs(cec.choose_device([a,b],1,3),b)
        for devices in ([],[a,b],[SimpleNamespace(idProduct=3,bus=1,address=2)]):
            with self.assertRaises(RuntimeError):
                cec.choose_device(devices)

    def test_wrong_and_truncated_firmware(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/"test.uf2"
            block=bytearray(512)
            struct.pack_into("<8I",block,0,0x0A324655,0x9E5D5157,0x2000,0x10000000,256,0,1,0xe48bff59)
            struct.pack_into("<I",block,508,0x0AB16F30)
            p.write_bytes(block)
            cec.validate_uf2(p)
            struct.pack_into("<I",block,28,0xe48bff56) # RP2040
            p.write_bytes(block)
            with self.assertRaises(ValueError): cec.validate_uf2(p)
            p.write_bytes(block[:-1])
            with self.assertRaises(ValueError): cec.validate_uf2(p)
            p.write_bytes(b"")
            with self.assertRaises(ValueError): cec.validate_uf2(p)

if __name__=="__main__":
    unittest.main()
