#include "button_scan.h"
#include <assert.h>
#include <stdio.h>

int main(void)
{
    assert(!button_scan_idle(false, false));
    assert(!button_scan_idle(false, true));
    assert(!button_scan_idle(true, true));
    assert(button_scan_idle(true, false));
    puts("Button scan idle policy tests: PASS");
}
