#include "util.h"
<<<<<<< Updated upstream
using namespace std;

int main(int argc, char * argv[])
{
=======
#include <condition_variable>
#include <cstdlib>
#include <exception>
#include <memory>
#include <mutex>
#include <signal.h>
#include <unistd.h>
using namespace std;



int main(int argc, char * argv[])
{
    PRINT(SIGSTKSZ);
    PRINT(MINSIGSTKSZ);
>>>>>>> Stashed changes

    return 0;
}
