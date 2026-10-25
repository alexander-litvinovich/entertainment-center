from pathlib import Path
import subprocess
import sys
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/"build/tests"
OUT.mkdir(parents=True,exist_ok=True)
commands=[
 ["cc","-std=c11","-Wall","-Wextra","-Werror","-g",
  "-I",str(ROOT/"firmware/include"),str(ROOT/"tests/test_command.c"),
  str(ROOT/"firmware/src/cec-command.c"),"-o",str(OUT/"command")],
 ["cc","-std=c11","-Wall","-Wextra","-Werror","-Wno-unused-parameter",
  "-g","-I",str(ROOT/"tests/stubs"),
  "-I",str(ROOT/"firmware/include"),str(ROOT/"tests/test_frame.c"),"-o",str(OUT/"frame")],
 [str(OUT/"command")],[str(OUT/"frame")],
 [sys.executable,"-m","unittest","discover","-s",str(ROOT.parent/".agents/skills/_shared"),"-p","test_*.py","-v"]]
for cmd in commands:
    subprocess.run(cmd,cwd=ROOT,check=True,timeout=30)
