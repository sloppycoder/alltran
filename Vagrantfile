#
#  this box is for testing headless Chrome on CentOS 7
#
$script = <<END_OF_SCRIPT

# enable EPEL
yum update -y
yum install -y epel-release
yum update -y

# disale SeLinux
sed -i 's/^SELINUX=.*/SELINUX=disabled/g' /etc/sysconfig/selinux


# other essential packages here
# yum install git

# install chrome and test if it's working
yum install -y --nogpgcheck https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm
su vagrant -c "cd /home/vagrant; /opt/google/chrome/chrome --headless --disable-gpu --screenshot https://www.chromestatus.com"

END_OF_SCRIPT

VAGRANTFILE_API_VERSION = "2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

	config.vm.box = "centos/7"
	config.vm.box_check_update = false

	config.vm.provider 'virtualbox' do |v|
	    v.memory = 1536
	    v.cpus = 1
	end

	# Set up a private IP in the Vagrant Environment. Important for Multi-Host Vagrant deployments.
	config.vm.network "private_network", ip: "192.168.50.10"

	config.vm.provision 'shell', inline: $script

end