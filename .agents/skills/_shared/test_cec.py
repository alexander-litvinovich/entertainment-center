import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

spec=importlib.util.spec_from_file_location("cec",Path(__file__).resolve().with_name("cec.py"))
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

class ReleaseDownload(unittest.TestCase):
    def test_latest_release_is_verified_and_saved(self):
        block=bytearray(512)
        struct.pack_into("<8I",block,0,0x0A324655,0x9E5D5157,0x2000,0x10000000,256,0,1,0xe48bff59)
        struct.pack_into("<I",block,508,0x0AB16F30)
        manifest=b'{"version":"cec-host-0.8.0-abc"}'
        sums=f"{hashlib.sha256(block).hexdigest()}  pico-cec.uf2\n{hashlib.sha256(manifest).hexdigest()}  manifest.json\n"
        def release(tag,prerelease=False):
            return {"tag_name":tag,"draft":False,"prerelease":prerelease,
                    "assets":[{"name":n,"browser_download_url":tag+"/"+n} for n in ("pico-cec.uf2","manifest.json","SHA256SUMS")]}
        files={"hdmi-cec-v0.10.0/pico-cec.uf2":bytes(block),"hdmi-cec-v0.10.0/manifest.json":manifest,
               "hdmi-cec-v0.10.0/SHA256SUMS":sums.encode()}
        releases=[release("hdmi-cec-v0.9.0"),release("hdmi-cec-v0.11.0",True),release("other-v1.0.0"),release("hdmi-cec-v0.10.0")]
        def fake(url):
            return json.dumps(releases).encode() if "api.github.com" in url else files[url]
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(cec,"github",fake), mock.patch.object(cec,"ROOT",Path(tmp)):
            cec.download(SimpleNamespace(tag=None))
            self.assertEqual((Path(tmp)/"build/releases/hdmi-cec-v0.10.0/manifest.json").read_bytes(),manifest)
            files["hdmi-cec-v0.10.0/manifest.json"]=b"tampered"
            with self.assertRaises(RuntimeError): cec.download(SimpleNamespace(tag=None))

if __name__=="__main__":
    unittest.main()
