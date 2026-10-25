#include <assert.h>
#include <stdio.h>
#include "cec-command.h"

int main(void) {
  cec_command_frame_t frame;
  assert(cec_command_encode(CEC_COMMAND_ON, 0, &frame));
  assert(frame.len == 2 && frame.data[0] == 0x40 && frame.data[1] == 0x04);
  assert(cec_command_encode(CEC_COMMAND_OFF, 0, &frame));
  assert(frame.len == 2 && frame.data[0] == 0x40 && frame.data[1] == 0x36);
  assert(cec_command_encode(CEC_COMMAND_INPUT, 0x2100, &frame));
  assert(frame.len == 4 && frame.data[0] == 0x4f && frame.data[1] == 0x86 &&
         frame.data[2] == 0x21 && frame.data[3] == 0x00);
  assert(cec_command_valid_physical_address(0x1000));
  assert(cec_command_valid_physical_address(0x2340));
  assert(!cec_command_valid_physical_address(0x0000));
  assert(!cec_command_valid_physical_address(0x2010));
  assert(!cec_command_encode(CEC_COMMAND_INPUT, 0x2010, &frame));
  assert(!cec_command_encode((cec_command_kind_t)99, 0, &frame));
  puts("CEC command encoding scenarios passed");
}
