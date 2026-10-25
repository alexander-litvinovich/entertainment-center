#include <assert.h>
#include <stdio.h>
#include "cec-command.h"

int main(void) {
  const uint8_t on[] = {0x40, 0x04};
  const uint8_t active[] = {0x4f, 0x82, 0x20, 0x00};
  const uint8_t poll[] = {0x44};
  const uint8_t other_initiator[] = {0x04, 0x36};
  uint8_t long_frame[CEC_COMMAND_MAX_LEN + 1] = {0x4f};
  assert(cec_command_valid_frame(on, sizeof(on)));
  assert(cec_command_valid_frame(active, sizeof(active)));
  assert(cec_command_valid_frame(poll, sizeof(poll)));
  assert(cec_command_valid_frame(long_frame, CEC_COMMAND_MAX_LEN));
  assert(!cec_command_valid_frame(long_frame, CEC_COMMAND_MAX_LEN + 1));
  assert(!cec_command_valid_frame(on, 0));
  assert(!cec_command_valid_frame(NULL, 1));
  assert(!cec_command_valid_frame(other_initiator, sizeof(other_initiator)));
  puts("CEC command encoding scenarios passed");
}
