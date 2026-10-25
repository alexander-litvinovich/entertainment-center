#include "pico/stdlib.h"
#include "cec-command.h"
#include "cec-task.h"

static cec_command_status_t command_status;
static bool command_available;

bool cec_command_submit(uint32_t id, const uint8_t *data, size_t len) {
  if (id == 0 || !cec_command_valid_frame(data, len)) return false;
  bool accepted = false;
  taskENTER_CRITICAL();
  if (!command_available && command_status.state != CEC_COMMAND_PENDING &&
      command_status.state != CEC_COMMAND_QUEUED) {
    command_status = (cec_command_status_t){.id = id, .state = CEC_COMMAND_QUEUED,
                                            .result = CEC_COMMAND_ERROR};
    for (size_t i = 0; i < len; ++i) command_status.frame.data[i] = data[i];
    command_status.frame.len = (uint8_t)len;
    command_available = true;
    accepted = true;
  }
  taskEXIT_CRITICAL();
  return accepted;
}

bool cec_command_snapshot(uint32_t id, cec_command_status_t *out) {
  if (!out) return false;
  bool found;
  taskENTER_CRITICAL();
  found = command_status.id == id && command_status.state != CEC_COMMAND_IDLE;
  if (found) *out = command_status;
  taskEXIT_CRITICAL();
  return found;
}

uint16_t cec_get_physical_address(void) { return 0xffff; }
uint8_t cec_get_logical_address(void) { return 4; }
bool cec_send_msg(const cec_message_t *msg) { (void)msg; return false; }

void cec_task(void *param) {
  (void)param;
  cec_frame_init();
  for (;;) {
    cec_command_status_t current = {0};
    bool execute = false;
    taskENTER_CRITICAL();
    if (command_available) {
      command_available = false;
      command_status.state = CEC_COMMAND_PENDING;
      current = command_status;
      execute = true;
    }
    taskEXIT_CRITICAL();

    if (execute) {
      cec_message_t message = {.len = current.frame.len};
      for (uint8_t i = 0; i < message.len; ++i) message.data[i] = current.frame.data[i];
      cec_command_result_t result =
          (cec_command_result_t)cec_frame_send_result(&message, 10000);
      taskENTER_CRITICAL();
      command_status.result = result;
      command_status.state = CEC_COMMAND_DONE;
      taskEXIT_CRITICAL();
    }
    vTaskDelay(pdMS_TO_TICKS(10));
  }
}
