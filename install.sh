#!/bin/bash

_() {
set -euo pipefail

initialiaze_file_descriptor() {
    # Hack: If the script is being read in from a pipe, then FD 0 is not the terminal input. But we
    # need input from the user! We just verified that FD 1 is a terminal, therefore we expect that
    # we can actually read from it instead. However, "read -u 1" in a script results in
    # "Bad file descriptor", even though it clearly isn't bad (weirdly, in an interactive shell,
    # "read -u 1" works fine). So, we clone FD 1 to FD 3 and then use that -- bash seems OK with
    # this.
    exec 3<&1
}

prompt() {
    local VALUE
  
    # Hack: We read from FD 3 because when reading the script from a pipe, FD 0 is the script, not
    # the terminal. We checked above that FD 1 (stdout) is in fact a terminal and then dup it to
    # FD 3, thus we can input from FD 3 here.
    # We use "bold", rather than any particular color, to maximize readability.
    echo -en '\e[1m' >&3
    echo -n "$1 [$2]" >&3
    echo -en '\e[0m ' >&3
    read -r -u 3 VALUE
    if [ -z "$VALUE" ]; then
        VALUE=$2
    fi
    echo "$VALUE"
}

prompt-yesno() {
    while true; do
        local VALUE
        VALUE=$(prompt "$@")
        case $VALUE in
            y | Y | yes | YES | Yes )
                return 0
                ;;
            n | N | no | NO | No )
                return 1
                ;;
        esac
        echo "*** Please answer \"yes\" or \"no\"."
    done
}

_curl() {
    # This function overrides the cURL command by
    # first printing ... to STDERR to indicate
    # that we are waiting.
    # Once the command completes, we clear the screen.
    echo -n '...' >&2
    curl --silent --fail "$@"
    echo -ne '\r' >&2
}

DEVICE="${DEVICE:-}"
BLOCK_DEVICE="${BLOCK_DEVICE:-}"
WORKING_DIRECTORY="${WORKING_DIRECTORY:-$(mktemp -d)}"
ONBOARD_PATH="${ONBOARD_PATH:-}"
OS_PATH="${OS_PATH:-}"
ARCH="${ARCH:-}"
ARCH_FULL="${ARCH_FULL:-}"
HOOKS="${HOOKS:-}"
BOOT_PARTITION=""
ROOT_PARTITION=""

determine_device() {
    if [ -n "$DEVICE" ]; then
        return
    fi
    while true; do
        echo "Onboard can be installed onto the following devices:"
        echo ""
        printf "\t(1) Raspberry Pi 4\n"
        printf "\t(2) Raspberry Pi 3\n"
        printf "\t(3) Banana Pi M2-Zero\n"
        echo ""
        case $(prompt "What kind of device would you like to install Onboard on?" 1) in
            1)
                DEVICE=rpi4
                ARCH=arm64
                ARCH_FULL=aarch64
                return
                ;;
            2)
                DEVICE=rpi3
                ARCH=arm64
                ARCH_FULL=aarch64
                return
                ;;
            3)
                DEVICE=bpim2z
                ARCH=arm
                ARCH_FULL=armv7h
                return
                ;;
            *)
                echo ""
                echo "Please choose from the options above."
                echo ""
        esac
    done
}

determine_block_device() {
    if [ -n "$BLOCK_DEVICE" ]; then
        return
    fi
    local DISCOVERED_BLOCK_DEVICES
    DISCOVERED_BLOCK_DEVICES=$(lsblk --output PATH,TYPE,HOTPLUG --noheadings | sed -n 's/\(\/dev\/[^\s]\+\)\s\+disk\s\+1/\1/p' | awk '{print $1}')
    while true; do
        local i=1
        echo "We need to install Onboard onto an SD card."
        echo "We found the following block devices:"
        echo ""
        for b in $DISCOVERED_BLOCK_DEVICES; do
            printf "\t(%s) %s\n" "$i" "$b"
            ((i++))
        done
        echo ""
        local VALUE
        VALUE=$(prompt "Which device is the SD card we should install to?" 0)
        case $VALUE in
            [1-"$(echo "$DISCOVERED_BLOCK_DEVICES" | wc -l)"])
                BLOCK_DEVICE=$(echo "$DISCOVERED_BLOCK_DEVICES" | sed -n "$VALUE"p)
                echo ""
                if prompt-yesno "Device $BLOCK_DEVICE was selected. Is this correct?" "yes"; then
                    return
                fi
                ;;
            *)
                echo ""
                echo "Please choose from the options above."
                echo ""
        esac
    done
}

determine_partitions() {
    local PARTITIONS
    PARTITIONS=$(lsblk "$BLOCK_DEVICE" --output PATH --noheadings | tail -n +2)
    BOOT_PARTITION=$(echo "$PARTITIONS" | sed -n 1p)
    if [ "$(echo "$PARTITIONS" | wc -l)" -eq 2 ]; then
        ROOT_PARTITION=$(echo "$PARTITIONS" | sed -n 2p)
    else
        ROOT_PARTITION=$(echo "$PARTITIONS" | sed -n 1p)
    fi
}

partition() {
    case $DEVICE in
        bpim2z)
            partition_bpim2z
            ;;
        rpi4)
            partition_rpi
            ;;
        rpi3)
            partition_rpi
            ;;
        *)
            echo ""
            echo "This is an unrecognized device; not sure how to proceed"
            exit 1
    esac
}

partition_bpim2z() {
    sudo dd if=/dev/zero of="$BLOCK_DEVICE" bs=1M count=8
    sudo parted -s "$BLOCK_DEVICE" mklabel msdos
    sudo parted -a optimal -- "$BLOCK_DEVICE" mkpart primary 2048s 100%
    determine_partitions
    sudo mkfs.ext4 -F -O ^metadata_csum,^64bit "$BOOT_PARTITION"
}

partition_rpi() {
    sudo dd if=/dev/zero of="$BLOCK_DEVICE" bs=1M count=8
    sudo parted -s "$BLOCK_DEVICE" mklabel msdos
    sudo parted -a optimal -- "$BLOCK_DEVICE" mkpart primary fat32 2048s 200M
    sudo parted -a optimal -- "$BLOCK_DEVICE" mkpart primary 200M 100%
    determine_partitions
    sudo mkfs.vfat "$BOOT_PARTITION"
    sudo mkfs.ext4 -F "$ROOT_PARTITION"
}

install_os() {
    mkdir -p "$WORKING_DIRECTORY"/root
    mkdir -p "$WORKING_DIRECTORY"/boot
    sudo mount "$BOOT_PARTITION" "$WORKING_DIRECTORY"/boot
    sudo mount "$ROOT_PARTITION" "$WORKING_DIRECTORY"/root
    case $DEVICE in
        bpim2z)
            install_os_bpim2z
            ;;
        rpi4)
            install_os_rpi https://archlinuxarm.org/os/ArchLinuxARM-rpi-aarch64-latest.tar.gz
            install_os_rpi4
            ;;
        rpi3)
            install_os_rpi https://archlinuxarm.org/os/ArchLinuxARM-rpi-aarch64-latest.tar.gz
            ;;
        *)
            echo ""
            echo "This is an unrecognized device; not sure how to proceed"
            exit 1
    esac
    sudo rm -f "$WORKING_DIRECTORY"/root/etc/machine-id
    sudo mkdir -p "$WORKING_DIRECTORY"/root/etc/systemd/system/systemd-firstboot.service.d
    cat <<EOF | sudo tee "$WORKING_DIRECTORY"/root/etc/systemd/system/systemd-firstboot.service.d/onboard.conf
[Service]
ExecStart=
ExecStart=systemd-firstboot
EOF
    sudo userdel --remove --prefix "$WORKING_DIRECTORY"/root alarm
    sudo useradd --password '*' --create-home --prefix "$WORKING_DIRECTORY"/root onboard
    sudo install --mode 0700 --owner 1000 --group 1000 --directory "$WORKING_DIRECTORY"/root/home/onboard/.ssh
    sudo umount "$WORKING_DIRECTORY"/boot
    sudo umount "$WORKING_DIRECTORY"/root
}

install_os_bpim2z() {
    pushd "$WORKING_DIRECTORY"
    if [ -z "$OS_PATH" ]; then
        echo "Downloading filesystem image"
        OS_PATH="$WORKING_DIRECTORY"/os.tar.gz
        _curl --location --output "$OS_PATH" https://archlinuxarm.org/os/ArchLinuxARM-armv7-latest.tar.gz 
    fi
    echo "Extracting filesystem image"
    sudo bsdtar -xpf "$OS_PATH" -C root
    local commandline=
    for h in $HOOKS; do
        local kcl
        # shellcheck disable=SC1090
        if kcl=$(. "$h" && kernel-command-line); then
            commandline="$commandline $kcl"
        fi
    done
    cat <<EOF > boot.cmd
part uuid \${devtype} \${devnum}:\${bootpart} uuid
setenv bootargs console=\${console} root=PARTUUID=\${uuid} rw rootwait$commandline

if load \${devtype} \${devnum}:\${bootpart} \${kernel_addr_r} /boot/zImage; then
  if load \${devtype} \${devnum}:\${bootpart} \${fdt_addr_r} /boot/dtbs/\${fdtfile}; then
    if load \${devtype} \${devnum}:\${bootpart} \${ramdisk_addr_r} /boot/initramfs-linux.img; then
      bootz \${kernel_addr_r} \${ramdisk_addr_r}:\${filesize} \${fdt_addr_r};
    else
      bootz \${kernel_addr_r} - \${fdt_addr_r};
    fi;
  fi;
fi

if load \${devtype} \${devnum}:\${bootpart} 0x48000000 /boot/uImage; then
  if load \${devtype} \${devnum}:\${bootpart} 0x43000000 /boot/script.bin; then
    setenv bootm_boot_mode sec;
    bootm 0x48000000;
  fi;
fi
EOF
    sudo mkimage -A arm -O linux -T script -C none -a 0 -e 0 -n "BananaM2Zero boot script" -d boot.cmd root/boot/boot.scr
    _curl --location https://source.denx.de/u-boot/u-boot/-/archive/v2021.04/u-boot-v2021.04.tar.gz | tar xvz
    pushd u-boot-*
    make -j4 ARCH="$ARCH" CROSS_COMPILE=arm-none-eabi- bananapi_m2_zero_defconfig
    make -j4 ARCH="$ARCH" CROSS_COMPILE=arm-none-eabi-
    sudo dd if=u-boot-sunxi-with-spl.bin of="$BLOCK_DEVICE" bs=1024 seek=8
    sync
}

install_os_rpi4() {
    pushd "$WORKING_DIRECTORY"
    sudo sed -i 's/mmcblk0/mmcblk1/g' root/etc/fstab
    sync
}

install_os_rpi() {
    pushd "$WORKING_DIRECTORY"
    if [ -z "$OS_PATH" ]; then
        echo "Downloading filesystem image"
        OS_PATH="$WORKING_DIRECTORY"/os.tar.gz
        _curl --location --output "$OS_PATH" "$1"
    fi
    echo "Extracting filesystem image"
    sudo bsdtar -xpf "$OS_PATH" -C root
    sudo mv root/boot/* boot
    sync
    local commandline=
    for h in $HOOKS; do
        local kcl
        # shellcheck disable=SC1090
        if kcl=$(. "$h" && kernel-command-line); then
            commandline="$commandline $kcl"
        fi
    done
    #sudo sed -i "1 s|$| $commandline|" boot/cmdline.txt
    sync
}

download_onboard() {
    if [ -n "$ONBOARD_PATH" ]; then
        return
    fi
    ONBOARD_PATH=$(mktemp -d)
    echo "Downloading Onboard"
    _curl --location https://github.com/squat/onboard/archive/main.tar.gz | sudo tar xvz --strip-components=1 -C "$ONBOARD_PATH" onboard-master/usr onboard-master/var
}

compile_onboard() {
    pushd "$ONBOARD_PATH"
    echo ""
    echo "Compiling the Onboard binary for $ARCH"
    echo ""
    make ARCH="$ARCH"
}

install_onboard() {
    pushd "$WORKING_DIRECTORY"
    sudo mount "$ROOT_PARTITION" root
    sudo cp -r "$ONBOARD_PATH"/fs/usr/lib/onboard root/usr/lib
    sudo cp "$ONBOARD_PATH"/bin/"$ARCH"/onboard root/usr/bin/onboard
    sudo mkdir -p root/var/lib/onboard
    echo "UUID=$(uuidgen)" | sudo tee root/var/lib/onboard/onboard.env > /dev/null
    sudo mkdir -p root/etc/onboard
    sudo ln -fs /usr/lib/onboard/00-wlan.yaml root/etc/onboard/
    sudo ln -fs /usr/lib/onboard/05-ssh.yaml root/etc/onboard/
    sudo ln -fs /usr/lib/onboard/50-onboard.preset root/usr/lib/systemd/system-preset/
    sudo ln -fs /usr/lib/onboard/ap@.service root/etc/systemd/system/
    sudo ln -fs /usr/lib/onboard/hostapd@.service root/etc/systemd/system/
    sudo ln -fs /usr/lib/onboard/hostapd-manager@.service root/etc/systemd/system/
    sudo ln -fs /usr/lib/onboard/hostapd-manager@ap0.path root/etc/systemd/system/hostapd-manager@ap0.path
    sudo ln -fs /usr/lib/onboard/onboard.service root/etc/systemd/system/
    sudo ln -fs /usr/lib/onboard/ping@.service root/etc/systemd/system/
    sudo ln -fs /usr/lib/onboard/ap.network root/etc/systemd/network/ap.network
    sudo ln -fs /usr/lib/onboard/wlan.network root/etc/systemd/network/wlan.network
    sudo ln -fs /usr/lib/onboard/wpa_supplicant@wlan0.path root/etc/systemd/system/wpa_supplicant@wlan0.path
    sudo ln -fs /usr/lib/onboard/sshd_config root/etc/ssh/sshd_config
    sudo touch root/etc/onboard/done-files
    for h in $HOOKS; do
        local df
        # shellcheck disable=SC1090
        if df=$(. "$h" && done-file); then
            echo "$df" | sudo tee --append root/etc/onboard/done-files > /dev/null
        fi
    done
    sudo umount root
}

install_wlan() {
    pushd "$WORKING_DIRECTORY"
    sudo mount "$ROOT_PARTITION" root
    echo "Downloading WLAN packages"
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/core/wpa_supplicant-2:2.9-8-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr etc
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/community/hostapd-2.9-5-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root var usr etc
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/core/iw-5.9-1-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr
    sudo umount root
}

install_hooks() {
    pushd "$WORKING_DIRECTORY"
    sudo mount "$ROOT_PARTITION" root
    for h in $HOOKS; do
        # shellcheck disable=SC1090
        (. "$h" && WORKING_DIRECTORY="$WORKING_DIRECTORY" ARCH="$ARCH" ARCH_FULL="$ARCH_FULL" install)
    done
    sudo umount root
}

initialiaze_file_descriptor
determine_device
determine_block_device
partition
determine_partitions
install_os
download_onboard
compile_onboard
install_onboard
install_wlan
install_hooks
}

_ "$0" "$@"
