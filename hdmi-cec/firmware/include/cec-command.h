#ifndef CEC_COMMAND_H
#define CEC_COMMAND_H
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#define CEC_COMMAND_MAX_LEN 16

typedef enum {
  CEC_COMMAND_IDLE,
  CEC_COMMAND_QUEUED,
  CEC_COMMAND_PENDING,
  CEC_COMMAND_DONE,
} cec_command_state_t;

typedef enum {
  CEC_COMMAND_ACK,
  CEC_COMMAND_NACK,
  CEC_COMMAND_BUSY,
  CEC_COMMAND_TIMEOUT,
  CEC_COMMAND_COLLISION,
  CEC_COMMAND_ERROR,
} cec_command_result_t;

typedef struct {
  uint8_t data[CEC_COMMAND_MAX_LEN];
  uint8_t len;
} cec_command_frame_t;

typedef struct {
  uint32_t id;
  cec_command_frame_t frame;
  cec_command_state_t state;
  cec_command_result_t result;
} cec_command_status_t;

// A frame is valid when it is 1..16 bytes long and sent from logical address 4.
bool cec_command_valid_frame(const uint8_t *data, size_t len);
bool cec_command_submit(uint32_t id, const uint8_t *data, size_t len);
bool cec_command_snapshot(uint32_t id, cec_command_status_t *out);
const char *cec_command_state_name(cec_command_state_t state);
const char *cec_command_result_name(cec_command_result_t result);
#endif
