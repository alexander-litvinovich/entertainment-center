#include "cec-command.h"

bool cec_command_valid_frame(const uint8_t *data, size_t len) {
  return data && len >= 1 && len <= CEC_COMMAND_MAX_LEN && (data[0] >> 4) == 4;
}

const char *cec_command_state_name(cec_command_state_t state) {
  switch (state) {
    case CEC_COMMAND_QUEUED: return "queued";
    case CEC_COMMAND_PENDING: return "pending";
    case CEC_COMMAND_DONE: return "done";
    default: return "idle";
  }
}

const char *cec_command_result_name(cec_command_result_t result) {
  switch (result) {
    case CEC_COMMAND_ACK: return "ack";
    case CEC_COMMAND_NACK: return "nack";
    case CEC_COMMAND_BUSY: return "busy";
    case CEC_COMMAND_TIMEOUT: return "timeout";
    case CEC_COMMAND_COLLISION: return "collision";
    default: return "error";
  }
}
