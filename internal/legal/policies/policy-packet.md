# Probably, Inc.

## Table of Contents

- [Access Control Policy](#access-control-policy)
- [Asset Management Policy](#asset-management-policy)
- [Business Continuity and Disaster Recovery Plan](#business-continuity-and-disaster-recovery-plan)
- [Code of Conduct](#code-of-conduct)
- [Cryptography Policy](#cryptography-policy)
- [Data Management Policy](#data-management-policy)
- [Human Resource Security Policy](#human-resource-security-policy)
- [Incident Response Plan](#incident-response-plan)
- [Information Security Policy (AUP)](#information-security-policy-aup)
- [Information Security Roles and Responsibilities](#information-security-roles-and-responsibilities)
- [Operations Security Policy](#operations-security-policy)
- [Physical Security Policy](#physical-security-policy)
- [Risk Management Policy](#risk-management-policy)
- [Secure Development Policy](#secure-development-policy)
- [Third-Party Management Policy](#third-party-management-policy)

---

# Access Control Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To limit access to information and information processing systems, networks, and facilities to authorized parties in accordance with business objectives.

## Scope

All Probably, Inc. information systems that process, store, or transmit confidential data as defined in the Probably, Inc. Data Management Policy. This policy applies to all employees of Probably, Inc. and to all external parties with access to Probably, Inc. networks and system resources.

## General requirements

Access to information computing resources is limited to personnel with a business requirement for such access. Access rights shall be granted or revoked in accordance with this Access Control Policy.

## Business requirements of Access Control Policy

Probably, Inc. shall determine the type and level of access granted to individual users based on the "principle of least privilege." This principle states that users are only granted the level of access absolutely required to perform their job functions, and is dictated by Probably, Inc.'s business and security requirements. Permissions and access rights not expressly granted shall be, by default, prohibited.

Probably, Inc.'s primary method of assigning and maintaining consistent access controls and access rights shall be through the implementation of Role-Based Access Control (RBAC). Wherever feasible, rights and restrictions shall be allocated to groups. Individual user accounts may be granted additional permissions as needed with approval from the system owner or authorized party.

All privileged access to production infrastructure shall use Multi-Factor Authentication (MFA).

## Access to networks and network services

The following security standards shall govern access to Probably, Inc. networks and network services:

- Technical access to Probably, Inc. networks must be formally documented including the standard role or approver, grantor, and date
- Only authorized Probably, Inc. employees and third-parties working off a signed contract or statement of work, with a business need, shall be granted access to the Probably, Inc. production networks and resources
- Probably, Inc. guests may be granted access to guest networks after registering with office staff without a documented request
- Remote connections to production systems and networks must be encrypted

## Customer access management

When configuring cross-account access using GCP IAM roles, you must use a value you generate for the external ID, instead of one provided by the customer, to ensure the integrity of the cross account role configuration. A partner-generated external ID ensures that malicious parties cannot impersonate a customer's configuration and enforces uniqueness and format consistency across all customers.

The external IDs used must be unique across all customers. Reusing external IDs for different customers does not solve the confused deputy problem and runs the risk of customer A being able to view data of customer B by using the role ARN of customer B along with the external ID of customer B.

Customers must not be able to set or influence external IDs. When the external ID is editable, it is possible for one customer to impersonate the configuration of another.

## User access management

Probably, Inc. requires that all personnel have a unique user identifier for system access, and that user credentials and passwords are not shared between multiple personnel. Users with multiple levels of access (e.g. administrators) should be given separate accounts for normal system use and for administrative functions wherever feasible. Root, service, and administrator accounts may use a password management system to share passwords for business continuity purposes only. Administrators shall only use shared administrative accounts as needed. If a password is compromised or suspected of compromise the incident should be escalated to the information security team immediately and the password must be changed.

## User registration and deregistration

Only authorized administrators shall be permitted to create new user IDs, and may only do so upon receipt of a documented request from authorized parties. User provisioning requests must include approval from data owners or Probably, Inc. management authorized to grant system access. Prior to account creation, administrators should verify that the account does not violate any Probably, Inc. security or system access control policies such as segregation of duties, fraud prevention measures, or access rights restrictions.

User IDs shall be promptly disabled or removed when users leave the organization or contract work ends in accordance with SLAs. User IDs shall not be reused.

## User access provisioning

- New employees and/or contractors are not to be granted access to any Probably, Inc. production systems until after they have completed all HR on-boarding tasks, which may include but is not limited to signed employment agreement, intellectual property agreement, and acknowledgement of Probably, Inc.'s information security policy
- Access should be restricted to only what is necessary to perform job duties
- No access may be granted earlier than official employee start date
- Access requests and rights modifications shall be documented in an access request ticket or email. No permissions shall be granted without approval from the system or data owner or management
- Records of all permission and privilege changes shall be maintained for no less than one year

## Management of privileged access

Probably, Inc. shall ensure that the allocation and use of privileged access rights are restricted and managed judiciously. The objective is to ensure that only authorized users, software components, and services are granted privileged access rights. Probably, Inc. will ensure that access and privileges conform to the following standard:

- **Identify and Validate Users:** Identify users who require privileged access for each system and process.
- **Allocate Privileged Rights:** Provision access rights basing allocations on specific needs and competencies, and adhering strictly to the access control policy.
- **Maintain Authorization Protocols:** maintain records of all privileged access allocations.
- **Enforce Strong Authentication:** Require MFA for all privileged access.
- **Prevent Generic Admin ID Usage:** prevent the usage of generic administrative user IDs
- **Ensure Logging and Auditing:** Log all privileged logins and activity

## User access reviews

Administrators shall perform access rights reviews of user, administrator, and service accounts on an annual basis to verify that user access is limited to systems that are required for their job function. Access reviews shall be documented.

Access reviews may include group membership as well as evaluations of any specific or exception-based permission. Access rights shall also be reviewed as part of any job role change, including promotion, demotion, or transfer within the company.

## Removal & adjustment of access rights

The access rights of all users shall be promptly removed upon termination of their employment or contract, or when rights are no longer needed due to a change in job function or role. The maximum allowable time period for access termination is 72 business hours.

## Access provisioning, deprovisioning, and change procedure

The Access Management Procedure for Probably, Inc. systems can be found in Appendix A to this policy.

## Segregation of duties

Conflicting duties and areas of responsibility shall be segregated to reduce opportunities for unauthorized or unintentional modification or misuse of Probably, Inc. assets. When provisioning access, care should be taken that no single person can access, modify or use assets without authorization or detection. The initiation of an event should be separated from its authorization. The possibility of collusion should be considered when determining access levels for individuals and groups.

## User responsibility for the management of secret authentication information

Control and management of individual user passwords is the responsibility of all Probably, Inc. personnel and third-party users. Users shall protect secret authentication information in accordance with the Information Security Policy.

## Password policy

- Where feasible, passwords for confidential systems shall be configured to have at least eight (8) or more characters, one upper case, one number
- Passwords shall be set to lock out after 6 failed attempts
- For manual password resets, a user's identity must be verified prior to changing passwords
- Do not use secret questions (place of birth, etc.) as a sole password reset requirement

## Information access restriction

Applications must restrict access to program functions and information to authorized users and support personnel in accordance with the defined access control policy. The level and type of restrictions applied by each application should be based on the individual application requirements, as identified by the data owner. The application-specific access control policy must also conform to Probably, Inc. policies regarding access controls and data management.

Prior to implementation, evaluation criteria are to be applied to application software to determine the necessary access controls and data policies. Assessment criteria include, but are not limited to:

- Sensitivity and classification of data.
- Risk to the organization of unauthorized access or disclosure of data
- The ability to, and granularity of, control(s) on user access rights to the application and data stored within the application
- Restrictions on data outputs, including filtering sensitive information, controlling output, and restricting information access to authorized personnel
- Controls over access rights between the evaluated application and other applications and systems
- Programmatic restrictions on user access to application functions and privileged instructions
- Logging and auditing functionality for system functions and information access
- Data retention and aging features

All unnecessary default accounts must be removed or disabled before making a system available on the network. Specifically, vendor default passwords and credentials must be changed on all Probably, Inc. systems, devices, and infrastructure prior to deployment. This applies to ALL default passwords, including but not limited to those used by operating systems, software that provides security services, application and system accounts, and Simple Network Management Protocol (SNMP) community strings where feasible.

## Secure log-on procedures

Secure log-on controls shall be designed and selected in accordance with the sensitivity of data and the risk of unauthorized access based on the totality of the security and access control architecture.

## Password management system

Systems for managing passwords should be interactive and assist Probably, Inc. personnel in maintaining password standards by enforcing password strength criteria including minimum length, and password complexity where feasible.

All storage and transmission of passwords is to be protected using appropriate cryptographic protections, either through hashing or encryption.

## Use of privileged utility programs

Use of utility programs, system files, or other software that might be capable of overriding system and application controls or altering system configurations must be restricted to the minimum personnel required. Systems are to maintain logs of all use of system utilities or alteration of system configurations. Extraneous system utilities or other privileged programs are to be removed or disabled as part of the system build and configuration process.

Management approval is required prior to the installation or use of any ad hoc or third-party system utilities.

## Access to program source code

Access to program source code and associated items, including designs, specifications, verification plans, and validation plans shall be strictly controlled in order to prevent the introduction of unauthorized functionality into software, avoid unintentional changes, and protect Probably, Inc. intellectual property.

All access to source code shall be based on business need and must be logged for review and audit.

## Exceptions

Requests for an exception to this Policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

## APPENDIX A — Access management procedure

### 1. Overview

This procedure outlines the process for managing access to company systems and resources, ensuring necessary access rights while maintaining security and compliance standards.

### 2. Initiation and Standard Access Provisioning

- **Onboarding Completion:** HR sends an email to the IT Service Desk upon completion of the employee onboarding process, generating service tickets for access.
- **Provisioning Access:** IT provisions access to all company-wide systems and engineering systems for Members of Technical Staff (MTS), including email, intranet, development environments, and collaboration tools.

### 3. Requesting Additional Access

- **Access Request:** Employees or managers submit requests for additional access through the IT Service Desk portal, including necessary details and justification.
- **Approval Process:** The request is reviewed and approved by the appropriate manager or system owner.

### 4. Provisioning and Notification of Approved Access

- **Provisioning:** IT provisions the approved access and updates the service ticket.
- **Notification:** IT notifies the employee and manager of the granted access, including any conditions or limitations.

### 5. Access Review and Revocation

- **Periodic Review:** IT conducts periodic reviews to ensure access is still required and appropriate.
- **Revocation:** When an employee changes roles or leaves the company, HR notifies IT to revoke access, updating the service tickets accordingly.

## APPENDIX B — Access matrix

| Role | Email | Google Workspace | Expense tool | CRM | App Infrastructure | Version Control | Build System | Vuln Scanner |
|------|-------|------------------|--------------|-----|-------------------|-----------------|--------------|--------------|
| Employee | x | x | x | x | | | | |
| Engineer | x | x | x | x | x | x | x | |
| Engineer Sprvs | x | x | x | x | x | x | x | x |
| Sales | x | x | x | x | x | | | |
| Sales Mgr | x | x | x | x | | | | |

---

# Asset Management Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To identify organizational assets and define appropriate protection responsibilities. To ensure that information receives an appropriate level of protection in accordance with its importance to the organization. To prevent unauthorized disclosure, modification, removal, or destruction of information stored on media.

## Scope

This policy applies to all Probably, Inc. owned or managed information systems.

## Inventory of assets

Assets associated with information and information processing facilities that store, process, or transmit classified information shall be identified and an inventory of these assets shall be created and maintained.

## Ownership of assets

Assets maintained in the inventory shall be owned by a specific individual or group within Probably, Inc..

## Acceptable use of assets

Rules for the acceptable use of information, assets, and information processing facilities shall be identified and documented in the Information Security Policy.

## Loss or theft of assets

All Probably, Inc. personnel must immediately report the loss of any information systems, including portable or laptop computers, smartphones, PDAs, authentication tokens (keyfobs, one-time-password generators, or personally owned smartphones or devices with a Probably, Inc. software authentication token installed) or other devices that can store and process or help grant access to Probably, Inc. data.

## Return of assets

All personnel and third-party users of Probably, Inc. equipment shall return all of the organizational assets within their possession upon termination of their employment, contract, or agreement.

## Handling of assets

Employees and users who are issued or handle Probably, Inc. equipment are expected to use reasonable judgment and exercise due care in protecting and maintaining the equipment.

Employees are responsible for ensuring that company equipment is secured and properly attended to whenever it is transported or stored outside of company facilities.

All mobile devices shall be handled in accordance with the Information Security Policy.

Excepting employee-issued devices, no company computer equipment or devices may be moved or taken off-site without appropriate authorization from management.

## Asset disposal & reuse

Company devices and media that stored or processed confidential data shall be securely disposed of when no longer needed. Data must be erased prior to disposal or reuse, using an approved technology in order to ensure that data is not recoverable. Or a Certificate of Destruction (COD) must be obtained for devices destroyed by a third-party service.

Please refer to NIST Special Publication 800-88 Revision 1 "Guidelines for Media Sanitization" in order to select which methods are appropriate.

## Customer asset return

Any physical assets owned by customers shall be promptly returned to the customer following service termination in accordance with the terms of contract or service agreement.

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Business Continuity and Disaster Recovery Plan

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

The purpose of this business continuity plan is to prepare Probably, Inc. in the event of service outages caused by factors beyond our control (e.g., natural disasters, man-made events), and to restore services to the widest extent possible in a minimum time frame.

## Scope

All Probably, Inc. IT systems that are business critical. This policy applies to all employees of Probably, Inc. and to all relevant external parties, including but not limited to Probably, Inc. consultants and contractors.

In the event of a loss of availability of a hosting service provider, the COO will determine an appropriate response strategy.

## General requirements

In the event of a major disruption to production services and a disaster affecting the availability and/or security of the Probably, Inc. office, senior managers and executive staff shall determine mitigation actions.

A disaster recovery test, including a test of backup restoration processes, shall be performed on an annual basis.

## Alternate work facilities

If the Probably, Inc. office becomes unavailable due to a disaster, all staff shall work remotely from their homes or any safe location.

## Communications and escalation

Executive staff and senior managers should be notified of any disaster affecting Probably, Inc. facilities or operations.

Communications shall take place over approved channels such as Slack, email.

Key communication contacts are maintained and made available to all employees.

## Roles and responsibilities

| Role | Responsibility |
|------|----------------|
| COO | The COO shall lead BC/DR efforts to mitigate losses and recover the corporate network and information systems. |
| Departmental Heads | Each department head shall be responsible for communications with their departmental staff and any actions needed to maintain continuity of their business functions. Departmental heads shall communicate regularly with executive staff and the IT Manager. |
| Managers | Managers shall be responsible for communicating with their direct reports and providing any needed assistance for staff to continue working from alternative locations. |
| VP of Global Support | The VP of Global Support, in conjunction with the CEO and CFO shall be responsible for any external and client communications regarding any disaster or business continuity actions that are relevant to customers and third parties. |
| Lead Open Source Engineer | The Lead Open Source Engineer, in conjunction with the VP of Global Support, shall be responsible for leading efforts to maintain continuity of Probably, Inc. services to customers during a disaster. |
| COO | The COO shall be responsible for internal communications to employees as well as any action needed to maintain physical health and safety of the workforce. The CHRO shall work with the IT Manager to ensure continuity of physical security at the Probably, Inc. office. |

## Continuity of critical services

Procedures for maintaining continuity of critical services in a disaster can be found in Appendix A.

Recovery Time Objectives (RTO) and Recovery Point Objects (RPO) can be found in Appendix B.

Strategy for maintaining continuity of services can be seen in the following table:

| Key business process | Continuity strategy |
|---------------------|---------------------|
| Customer (Production) Service Delivery | Rely on GCP availability commitments and SLAs |
| IT Operations | Not dependent on HQ. Critical data is backed up to alternate locations. |
| Email | Utilize Gmail and its distributed nature, rely on Google's standard service level agreements. |
| Finance, Legal and HR | All systems are vendor-hosted SaaS applications. |
| Sales and Marketing | All systems are vendor-hosted SaaS applications. |

## Plan activation

This BC/DR shall be automatically activated in the event of the loss or unavailability of the Probably, Inc. office, or a natural disaster (i.e., severe weather, regional power outage, earthquake) affecting the larger region.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

## Appendix A - Business continuity procedures by scenario

### Business Continuity Scenarios

#### HQ Offline (power and/or network)

- CRM, Telephony, Video Conferencing/Screen Share & Corp Email unaffected
- SUPPORT unaffected
- HQ Staff offline (30-60 minutes)
- Remote Staff unaffected (US)

**Procedure:**
1. HQ Staff relocate to home offices (30-60 minutes)
2. Verify Telephony, CRM, & Email Connectivity at home offices (10 minutes)
3. Remotely resume normal operations

#### Colo Offline (power and/or network)

- CRM, Telephony, Video Conferencing/Screen Share & Corp Email unaffected
- SUPPORT Offline
- Production Database offline (redundant)
- HQ Staff unaffected
- Remote Staff unaffected (US)

**Procedure:**
1. Notify Customer Base that proactive monitoring is offline
2. Normal operations continue

#### Disaster Event at HQ

- CRM, Telephony, Video Conferencing/Screen Share & Corp Email unaffected
- SUPPORT offline
- HQ Staff offline (variable impact)
- Remote Staff unaffected (US)

**Procedure:**
1. Activate Remote Staff (US)
2. Notify Customer Base of impaired functions & potential delays
3. Commandeer Field Resources for Critical Response (SE Teams)

#### SaaS Tools Down

- CRM, Telephony, Video Conferencing/Screen Share, or Corp Email Affected
- SUPPORT partially affected (no new cases, manual triage required)
- HQ Staff unaffected
- Remote Staff unaffected (US)

**Procedure:**

**Telephony Down**
1. Notify Customer Base to use Support Portal or Email
2. Support Staff use Mobile Phones and/or Land Lines as needed

**Email Down (Gmail/Corp Email)**
1. Support Staff manually manage 'case' related communications
2. Support Staff use alternate email accounts as needed (Hotmail)

**CRM Down**
1. Notify Customer Base that CRM is down
2. Activate 'Spreadsheet' Case Tracking (Google Sheets)
3. Leverage 'Production' Database for Entitlements, Case History, Configuration data.

**Video Conferencing/ScreenShare Down (Zoom)**
1. Support Staff utilize alternate service as needed

## Appendix B - RTOs/RPOs

| Rank | Asset | Affected Assets | Business Impact | Users | Owners | Recovery Time Objective (RTO) | Recovery Point Objective (RPO) | Comments / Gaps |
|------|-------|-----------------|-----------------|-------|--------|------------------------------|-------------------------------|-----------------|
| 1 | Google Datacenters | Site | Core services | All | Engineering | 2 hours | 15 min | |
| 2 | Corporate Office | Site | Inability to access data? Any other impacts? | All | IT Ops | | | |
| | Corporate Network | Network | Inability to use network resources from corporate office | All | IT Ops | | | |
| | Google Cloud | Network | Core services | All | Engineering | | | |
| | Home Office ISP | Networks | Network | | IT Ops, Development | N/A | | |
| | Subcontractor Networks | Network | | | Development | N/A | | |
| | Third Party Networks | Network | | | Sales | N/A | | |
| | Company Laptops | Hardware | | All | IT Ops | | | |
| | Digital Projector | Hardware | | All | IT Ops | | | |
| | Office Printers | Hardware | Inability to print in corporate office | All | IT Ops | | | |
| | Personal Mobile Device | Hardware | | | | | | |
| | Wireless Access Points (WAP) | Hardware | | All | IT Ops | | | |

---

# Code of Conduct

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 11, 2025

## Purpose

The primary goal of Probably, Inc.'s Code of Conduct is to foster inclusive, collaborative and safe working conditions for all Probably, Inc. staff. As such, Probably, Inc. is committed to providing a friendly, safe and welcoming environment for all staff, regardless of gender, sexual orientation, ability, ethnicity, socioeconomic status, or religion (or lack thereof).

This code of conduct outlines our expectations for all Probably, Inc. staff, as well as the consequences for unacceptable behavior.

## Scope

The Code of Conduct applies to all Probably, Inc. staff. This includes full-time, part-time and contractor staff employed at every seniority level. The Code of Conduct is to be upheld during all professional functions and events, including but not limited to business hours at the Probably, Inc. office, during Probably, Inc.-related extracurricular activities and events, while attending conferences and other professional events on behalf of Probably, Inc., and while working remotely and communicating on Probably, Inc. resources with other staff.

We expect all Probably, Inc. staff to abide by this Code of Conduct in all business matters -- online and in-person -- as well as in all one-on-one communications with customers and staff pertaining to Probably, Inc. business.

This Code of Conduct also applies to unacceptable behavior occurring outside the scope of business activities when such behavior has the potential to adversely affect the safety and well-being of Probably, Inc. staff and clients.

## Culture and citizenship

A supplemental goal of this Code of Conduct is to increase open citizenship by encouraging participants to recognize the relationships between our actions and their effects within Probably, Inc. culture.

**Be welcoming.** We strive to be a company that welcomes and supports people of all backgrounds and identities. This includes, but is not limited to members of any race, ethnicity, culture, national origin, color, immigration status, social and economic class, educational level, sexual orientation, gender identity and expression, age, size, family status, political belief, religion, and mental and physical ability.

**Be considerate.** Your work at Probably, Inc. will be used by other people, and you in turn will depend on the work of others. Any decision you take will affect users and colleagues, and you should take those consequences into account when making decisions.

**Be respectful.** Not all of us will agree all the time, but disagreement is no excuse for poor behavior and poor manners. We might all experience some frustration now and then, but we cannot allow that frustration to turn into a personal attack. It's important to remember that a company where people feel uncomfortable or threatened is neither productive nor pleasant. Probably, Inc. staff should always be respectful when dealing with other personnel as well as with people outside of Probably, Inc. employment.

## Acceptable and expected behavior

The following behaviors are expected and requested of all Probably, Inc. staff:

- Participate in an authentic and active way. In doing so, you contribute to the health and longevity of Probably, Inc..
- Exercise consideration and respect in your speech and actions at all times.
- Attempt collaboration before conflict.
- Refrain from demeaning, discriminatory, or harassing behavior and speech.
- Be mindful of your surroundings and of your fellow participants. Alert Probably, Inc. leaders if you notice a dangerous situation, someone in distress, or violations of this Code of Conduct, even if they seem inconsequential.
- Remember that Probably, Inc. events may be shared with members of the public and Probably, Inc. customers; please be respectful to all patrons of these locations at all times

## Unacceptable behavior

The following behaviors are considered harassment and are unacceptable within our community:

- Violence, threats of violence or violent language directed against another person.
- Sexist, racist, homophobic, transphobic, ableist or otherwise discriminatory jokes and language.
- Posting or displaying sexually explicit or violent material.
- Posting or threatening to post other people's personally identifying information ("doxing").
- Personal insults, particularly those related to gender, sexual orientation, race, religion, or disability.
- Inappropriate photography or recording.
- Inappropriate physical contact. You should have someone's consent before touching them in any manner.
- Unwelcome sexual attention. This includes sexualized comments or jokes; inappropriate touching, groping, and unwelcome sexual advances.
- Deliberate intimidation, stalking or following (online or in person).
- Advocating for, or encouraging, any of the above behavior.
- Repeated harassment of others. In general, if someone asks you to stop, then stop.
- Other conduct which could reasonably be considered inappropriate in a professional setting.

## Weapons policy

No weapons will be allowed at Probably, Inc. events, office locations, or in other spaces covered by the scope of this Code of Conduct. Weapons include but are not limited to guns, explosives (including fireworks), and large knives such as those used for hunting or display, as well as any other item used for the purpose of causing injury or harm to others.

Anyone seen in possession of one of these items will be asked to leave immediately and will be subject to punitive action up to and including termination and involvement of law enforcement authorities. Probably, Inc. staff are further expected to comply with all state and local laws on this matter.

## Consequences of unacceptable behavior

Unacceptable behavior from any Probably, Inc. staff, including those with decision-making authority, will not be tolerated.

Anyone asked to stop unacceptable behavior is expected to comply immediately.

If a staff member engages in unacceptable behavior, Probably, Inc. leadership may take any action deemed appropriate, up to and including suspension or termination.

## Reporting violations

If you are subject to or witness unacceptable behavior, or have any other concerns, please notify an appropriate member of Probably, Inc. leadership as soon as possible.

It is a violation of this policy to retaliate against any person making a complaint of Unacceptable Behavior or against any person participating in the investigation of (including testifying as a witness to) any such allegation. Any retaliation or intimidation may be subject to punitive action up to and including termination.

## Disciplinary action

Personnel who violate this policy may face disciplinary consequences in proportion to their violation. Probably, Inc. management will determine how serious an employee's offense is and take the appropriate action.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Cryptography Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To ensure proper and effective use of cryptography to protect the confidentiality, authenticity and/or integrity of information. This policy establishes requirements for the use and protection of cryptographic keys and cryptographic methods throughout the entire encryption lifecycle.

## Scope

All information systems developed and/or controlled by Probably, Inc. which store or transmit confidential data.

## General requirements

Probably, Inc. shall evaluate the risks inherent in processing and storing data, and shall implement cryptographic controls to mitigate those risks where deemed appropriate. Where encryption is in use, strong cryptography with associated key management processes and procedures shall be implemented and documented. All encryption shall be performed in accordance with industry standards, including NIST SP 800-57.

Customer or confidential company data must utilize strong ciphers and configurations in accordance with vendor recommendations and industry best practices including NIST when stored or transferred over a public network.

## Key management

Access to keys and secrets shall be tightly controlled in accordance with the Access Control Policy.

The following table details Probably, Inc.'s approved encryption algorithms:

| Domain | Key Type | Algorithm | Key Length | Max Expiration |
|--------|----------|-----------|------------|----------------|
| Web Certificate | RSA or ECC with SHA2+ signature | RSA or ECC with SHA2+ signature | 2048 bit or greater/RSA, 256bit or greater/ECC | Up to 1 year |
| Web Cipher (TLS) | Asymmetric Encryption | Ciphers of B or greater grade on SSL Labs Rating | Varies | N/A |
| Confidential Data at Rest | Symmetric Encryption | AES | 256 bit | 1 Year |
| Passwords | One-way Hash | Bcrypt, PBKDF2, or scrypt, Argon2 | 256 bit+10K Stretch. Include unique cryptographic salt+pepper | N/A |
| Endpoint Storage (SSD/HDD) | Symmetric Encryption | AES | 128 or 256 bit | N/A |

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

A documented exception is required prior to moving, copying, or storing customer or company confidential data on any media or removable device; all portable devices and removable media containing sensitive data must be encrypted using approved standards and mechanisms.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Data Management Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To ensure that information is classified, protected, retained and securely disposed of in accordance with its importance to the organization.

## Scope

All Probably, Inc. data, information and information systems.

## General requirements

Probably, Inc. classifies data and information systems in accordance with legal requirements, sensitivity, and business criticality in order to ensure that information is given the appropriate level of protection. Data owners are responsible for identifying any additional requirements for specific data or exceptions to standard handling requirements.

Information systems and applications shall be classified according to the highest classification of data that they store or process.

## Data classification

To help Probably, Inc. and its employees easily understand requirements associated with different kinds of information, the company has created three classes of data.

### Confidential

Highly sensitive data requiring the highest levels of protection; access is restricted to specific employees or departments, and these records can only be passed to others with approval from the data owner, or a company executive. Examples include:

- Customer Data
- Personally identifiable information (PII)
- Company financial and banking data
- Salary, compensation and payroll information
- Strategic plans
- Incident reports
- Risk assessment reports
- Technical vulnerability reports
- Authentication credentials
- Secrets and private keys
- Source code
- Litigation data

### Restricted

Probably, Inc. proprietary information requiring thorough protection; access is restricted to personnel with a "need-to-know" based on business requirements. This data can only be distributed outside the company with approval. This is default for all company information unless stated otherwise. Examples include:

- Internal policies
- Legal documents
- Meeting minutes and internal presentations
- Contracts
- Internal reports
- Slack messages
- Email

### Public

Documents intended for public consumption which can be freely distributed outside Probably, Inc.. Examples include:

- Marketing materials
- Product descriptions
- Release notes
- External facing policies

## Labeling

Confidential data should be labeled "confidential" whenever paper copies are produced for distribution.

## Data handling

### Confidential Data Handling

Confidential data is subject to the following protection and handling requirements:

- Access for non-preapproved roles requires documented approval from the data owner
- Access is restricted to specific employees, roles and/or departments
- Confidential systems shall not allow unauthenticated or anonymous access
- Confidential Customer Data shall not be used or stored in non-production systems/environments
- Confidential data shall be encrypted at rest and in transit over public networks in accordance with the Cryptography Policy
- Mobile device hard drives containing confidential data, including laptops, shall be encrypted
- Mobile devices storing or accessing confidential data shall be protected by a log-on password (or equivalent, such as biometric) or passcode and shall be configured to lock the screen after five (5) minutes of non-use
- Backups shall be encrypted
- Confidential data shall not be stored on personal phones or devices or removable media including USB drives, CD's, or DVD's
- Paper records shall be labeled "confidential" and securely stored and disposed of in a secure, approved manner in accordance with data handling and destruction policies and procedures
- Hardcopy paper records shall only be created based on a business need and shall be avoided whenever possible
- Hard drives and mobile devices used to store confidential information must be securely wiped prior to disposal or physically destroyed
- Transfer of confidential data to people or entities outside the company shall only be done in accordance with a legal contract or arrangement, and the explicit written permission of management or the data owner

### Restricted Data Handling

Restricted data is subject to the following protection and handling requirements:

- Access is restricted to users with a need-to-know based on business requirements
- Restricted systems shall not allow unauthenticated or anonymous access
- Transfer of restricted data to people or entities outside the company or authorized users shall require management approval and shall only be done in accordance with a legal contract or arrangement, or the permission of the data owner
- Paper records shall be securely stored and disposed of in a secure, approved manner in accordance with data handling and destruction policies and procedures
- Hard drives and mobile devices used to store restricted information must be securely wiped prior to disposal or physically destroyed

### Public Data Handling

No special protection or handling controls are required for public data. Public data may be freely distributed.

## Data retention

Probably, Inc. shall retain data as long as the company has a need for its use, or to meet regulatory or contractual requirements. Once data is no longer needed, it shall be securely disposed of or archived. Data owners, in consultation with legal counsel, may determine retention periods for their data.

Personally identifiable information (PII) shall be deleted or de-identified as soon as it no longer has a business use.

Retention periods shall be documented in the Data Retention Matrix in Appendix B to this policy.

## Data & device disposal

Data classified as restricted or confidential shall be securely deleted when no longer needed. Probably, Inc. shall assess the data and disposal practices of third-party vendors in accordance with the Third-Party Management Policy. Only third-parties who meet Probably, Inc. requirements for secure data disposal shall be used for storage and processing of restricted or confidential data.

Probably, Inc. shall ensure that all restricted and confidential data is securely deleted from company devices prior to, or at the time of, disposal. Confidential and Restricted hardcopy materials shall be shredded or otherwise disposed of using a secure method.

Personally identifiable information (PII) shall be collected, used and retained only for as long as the company has a legitimate business purpose. PII shall be securely deleted and disposed of following contract termination in accordance with company policy, contractual commitments and all relevant laws and regulations. PII shall also be deleted in response to a verified request from a consumer or data subject, where the company does not have a legitimate business interest or other legal obligation to retain the data.

## Annual data review

Management shall review data retention requirements during the annual review of this policy. Data shall be disposed of in accordance with this policy.

## Legal requirements

Under certain circumstances, Probably, Inc. may become subject to legal proceedings requiring retention of data associated with legal holds, lawsuits, or other matters as stipulated by Probably, Inc. legal counsel. Such records and information are exempt from any other requirements specified within this Data Management Policy and are to be retained in accordance with requirements identified by the Legal department. All such holds and special retention requirements are subject to annual review with Probably, Inc.'s legal counsel to evaluate continuing requirements and scope.

## Policy compliance

Probably, Inc. will measure and verify compliance to this policy through various methods, including but not limited to, business tool reports, and both internal and external audits.

## Exceptions

Requests for an exception to this Policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

## APPENDIX A - Internal retention and disposal procedure

Probably, Inc.'s Engineering Team is responsible for setting and enforcing the data retention and disposal procedures for Probably, Inc. managed accounts and devices.

### Customer Accounts:

1. Customer accounts and data shall be deleted within sixty (60) days of contract termination through manual data deletion processes.

### Devices:

1. Employee devices will be collected promptly upon an employee's termination. Remote employees will be sent a shipping label and the return of their device shall be monitored.
2. Collected devices will be cleared to be re-provisioned - or removed from inventory, Probably, Inc. will securely erase the device when reprovisioning.
3. Device images may be retained at the discretion of management for business purposes

### Destroying devices or electronic media

In cases where a device is damaged in a way that Probably, Inc. cannot access the Recovery Partition to erase the drive, Probably, Inc. may optionally decide to use an E-Waste service that includes data destruction with a certificate. Probably, Inc. will keep certificates of destruction on record for one year.

Physical destruction can be optional if it is verified that the device is encrypted with Full Disk Encryption, which would negate the risk of data recovery.

Management will review this procedure at least annually.

## APPENDIX B - Data retention matrix

| System or Application | Data Description | Retention Period |
|----------------------|------------------|------------------|
| Probably, Inc. SaaS Products | Customer Data | Up to 60 days after contract termination |
| Probably, Inc. AutoSupport | Customer instance and metadata, debugging data | Indefinite |
| Probably, Inc. Customer Support Tickets | Support Tickets and Cases | Indefinite |
| Probably, Inc. Customer Support Phone Conversations | Support Phone Conversations | Indefinite |
| Probably, Inc. Security Event Data | Security and system event and log data, network data flow logs | On-Premise - Indefinite, GCP Instance - 1 year |
| Probably, Inc. Vulnerability Scan Data | Vulnerability scan results and detection data | 6 months host (asset) data is retained until removed and purged |
| Probably, Inc. Customer | Sales Opportunity and Sales Data | Indefinite |
| Probably, Inc. QA and Testing Data | QA, testing scenarios and results data | Indefinite |
| Security Policies | Security Policies | 1 year after archive |
| Temporary Files | temp ephemeral storage | automatically when process finishes |

---

# Human Resource Security Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To ensure that personnel and contractors meet security requirements, understand their responsibilities, and are suitable for their roles.

## Scope

This policy applies to all employees of Probably, Inc., consultants, contractors and other third-party entities with access to Probably, Inc. production networks and system resources.

## Screening

Background verification checks on Probably, Inc. personnel shall be carried out in accordance with relevant laws, regulations, and shall be proportional to the business requirements, the classification of the information to be accessed, and the perceived risks. Background screening shall include criminal history checks unless prohibited by local statute. All third-parties with technical privileged or administrative access to Probably, Inc. production systems or networks are subject to a background check or requirement to provide evidence of an acceptable background, based on their level of access and the perceived risk to Probably, Inc..

## Competence & performance assessment

The skills and competence of employees and contractors shall be assessed by human resources staff and the hiring manager or his or her designees as part of the hiring process. Required skills and competencies shall be listed in job descriptions and requisitions, and/or aligned with the responsibilities outlined in the Information Security Roles and Responsibilities Policy. Competency evaluations may include reference checks, education and certification verifications, technical testing, and interviews.

All Probably, Inc. employees will undergo an annual performance review which will include an assessment of job performance, competence in the role, adherence to company policies and code of conduct, and achievement of role-specific objectives.

## Terms & conditions of employment

Company policies and information security roles and responsibilities shall be communicated to employees and third-parties at the time of hire or engagement, and employees and contractors are required to formally acknowledge their understanding and acceptance of their security responsibilities. Employees and third-parties with access to company or customer information shall sign an appropriate non-disclosure, confidentiality, and appropriate code-of-conduct agreements. Contractual agreements shall state responsibilities for information security as needed. Employees and relevant third-parties shall follow all Probably, Inc. information security policies.

## Management responsibilities

Management shall be responsible for ensuring that information security policies and procedures are reviewed annually, distributed and available, and that personnel and contractors abide by those policies and procedures for the duration of their employment or engagement. Annual policy review shall include a review of any linked or referenced procedures, standards or guidelines.

Management shall ensure that information security responsibilities are communicated to individuals, through written job descriptions, policies or some other documented method which is accurately updated and maintained. Compliance with information security policies and procedures and fulfillment of information security responsibilities shall be evaluated as part of the performance review process wherever applicable.

Management shall consider excessive pressures, and opportunities for fraud when establishing incentives and segregating roles, responsibilities, and authorities.

## Information Security awareness, education & training

All Probably, Inc. employees and third-parties with administrative or privileged technical access to Probably, Inc. production systems and networks shall complete security awareness training at the time of hire and annually thereafter. Management shall monitor training completion and shall take appropriate steps to ensure compliance with this policy. Employees and contractors shall be aware of relevant information security and data privacy policies and procedures. The company shall ensure that personnel receive security and data privacy training appropriate to their role and data handling responsibilities.

In order to maintain a robust level of security awareness, the company will provide security-related updates and communications to company personnel on an on-going basis through multiple communication channels as needed.

Information security leaders and managers shall ensure appropriate professional development occurs to provide an understanding of current threats and trends in the security landscape. Security leaders and key stakeholders shall attend trainings, obtain and maintain relevant certifications, and maintain memberships in industry groups as appropriate.

## Termination process

Employee and contractor termination and offboarding processes shall ensure that physical and logical access is promptly revoked in accordance with company SLAs and policies, and that all company issued equipment is returned.

Any security or confidentiality agreements which remain valid after termination shall be communicated to the employee or contractor at time of termination.

## Disciplinary process

Employees and third-parties who violate Probably, Inc. information security policies shall be subject to the Probably, Inc. progressive disciplinary process, up to and including termination of employment or contract.

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company policies up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Incident Response Plan

**Policy Owner:** Kate Eaglen  
**Effective Date:** Sep 24, 2025

## Purpose

This document establishes the plan for managing information security incidents and events, and offers guidance for employees or incident responders who believe they have discovered, or are responding to, a security incident.

## Scope

This policy covers all information security or data privacy events or incidents.

## Incident and event definitions

A **security event** is an observable occurrence relevant to the confidentiality, availability, integrity, or privacy of company controlled data, systems or networks.

A **security incident** is a security event which results in loss or damage to the confidentiality, availability, integrity, or privacy of company controlled data, systems or networks.

## Reporting

If a Probably, Inc. employee, contractor, user, or customer becomes aware of an information security event or incident, possible incident, imminent incident, unauthorized access, policy violation, security weakness, or suspicious activity, then they shall immediately report the information using one of the following communication channels:

- Email security@probably.money information or reports about the event or incident

Reporters should act as a good witness and behave as if they are reporting a crime. Reports should include specific details about what has been observed or discovered.

## Severity

The engineering team shall monitor incident and event tickets and shall assign a ticket severity based on the following categories.

### P2/P3 - Medium and Low Severity

Issues meeting this severity are simply suspicions or odd behaviors. They are not verified and require further investigation. There is no clear indicator that systems have tangible risk and do not require emergency response. This includes lost/stolen laptop with disk encryption, suspicious emails, outages, strange activity on a laptop, etc.

### P1 - High Severity

High severity issues relate to problems where an adversary or active exploitation hasn't been proven yet, and may not have happened, but is likely to happen. This may include lost/stolen laptop without encryption, vulnerabilities with direct risk of exploitation, threats with risk or adversarial persistence on our systems (e.g., backdoors, malware), malicious access of business data (e.g., passwords, vulnerability data, payments information).

### P0 - Critical Severity

Critical issues relate to actively exploited risks and involve a malicious actor or threats that put any individual at risk of physical harm. Identification of active exploitation is required to meet this severity category.

## Escalation and internal reporting

The incident escalation contacts can be found below in Appendix A.

| Severity | Escalation Path |
|----------|-----------------|
| P0 - Critical Severity | P0 issues require immediate notification to IT and/or Engineering management. |
| P1 - High Severity | A support ticket must be created and the appropriate manager (see P0 above) must also be notified via email or Slack with a reference to the ticket number. |
| P2/P3 - Medium and Low Severity | A support ticket must be created and assigned to the appropriate department for response. |

## Documentation

All reported security events, incidents, and response activities shall be documented and adequately protected.

A root cause analysis may be performed on all verified P0 security incidents. A root cause analysis report shall be documented and referenced in the incident ticket. The root cause analysis shall be reviewed by the CEO who shall determine if a post-mortem meeting will be called.

## Incident response process

For critical issues, the response team will follow an iterative response process designed to investigate, contain exploitation, eradicate the threat, recover system and services, remediate vulnerabilities, and document a post-mortem report including the lessons learned from the incident.

### Summary

1. Event reported
2. Triage and analysis
3. Investigation
4. Containment & neutralization (short term/triage)
5. Recovery & vulnerability remediation
6. Hardening & Detection improvements (lessons learned, long term response)

### Detailed

- IT Manager or VP of Support will manage the incident response effort
- If necessary, a central "War Room" will be designated, which may be a physical or virtual location (i.e Slack channel)
- A recurring Incident Response Meeting will occur at regular intervals until the incident is resolved
- Legal and executive staff will be informed as required

### Incident response meeting agenda

- Update Incident Ticket and timelines
- Document new Indicators of Compromise (IOCs)
- Perform investigative Q&A
- Apply emergency mitigations
- External Reporting / Breach Reporting
- Plan long term mitigations
- Document Root Cause Analysis (RCA)
- Additional items as needed

## Special considerations

### Internal issues

Issues where the malicious actor is an internal employee, contractor, vendor, or partner requires sensitive handling. The incident manager shall contact CEO directly and will not discuss with other employees. These are critical issues where follow-up must occur.

### Compromised communication

Incident responders must have Slack messaging arranged before listing themselves as part of the incident response team. If there are IT communication risks, an out of band solution will be chosen, and communicated to incident responders via cell phone.

### Root account compromise

If an GCP root account compromise is known or expected, refer to the playbook in Appendix D.

## Additional requirements

- Suspected and reported events and incidents shall be documented
- Suspected incidents shall be assessed and classified as either an event or an incident
- Incident response shall be performed according to this plan and any associated procedures
- All incidents shall be formally documented, and a documented root cause analysis shall be performed
- Incident responders shall collect, store, and preserve incident-related evidence in accordance with industry guidance and best practices such as NIST SP 800-86 'Guide to Integrating Forensic Techniques into Incident Response'
- Suspected and confirmed unauthorized access events shall be reviewed by the Incident Response Team. Breach determinations shall only be made by the CEO
- Probably, Inc. shall promptly and properly notify customers, partners, users, affected parties, and regulatory agencies of relevant incidents or breaches in accordance with Probably, Inc. policies, contractual commitments, and regulatory requirements, as determined by the CEO
- This Incident Response Plan shall be reviewed and formally tested at least annually. Results of IR plan testing activities including findings and lessons learned will be formally documented and maintained to support security, compliance and audit requirements

## External communications and breach reporting

Legal and executive staff shall confer with technical teams in the event of unauthorized access to company or customer systems, networks, and/or data. Legal staff along with the CEO shall determine if breach reporting or external communications are required. Breaches shall be reported to customers, consumers, data subjects and regulators without undue delay and in accordance with all contractual commitments and applicable legislation.

No personnel may disclose information regarding incident or potential breaches to any third party or unauthorized person without the approval of legal and/or executive management.

## Mitigation and remediation

Legal and executive staff shall determine any immediate or long term mitigations or remedial actions that need to be taken as a result of an incident or breach. In the event that mitigations or remedial actions are needed, executive staff shall direct personnel with respect to planning, communicating and executing those activities.

## Cooperation with customers, Data Controller, and authorities

As needed and determined by legal and executive staff, the company shall cooperate with customers, Data Controllers and regulators to fulfill all of its obligations in the event of an incident or data breach.

## Roles & responsibilities

Every employee and user of any Probably, Inc. information resources has responsibilities toward the protection of the information assets. The table below establishes the specific responsibilities of the incident responder roles.

### Response Team Members

| Role | Responsibility |
|------|----------------|
| Incident Manager | The Incident Manager is the primary and ultimate decision maker during the response period. The Incident Manager is ultimately responsible for resolving the incident and formally closing incident response actions. See Appendix A for Incident Manager contact information. These responsibilities include: Ensuring the right people from all functions are actively involved as appropriate; Communicating status updates to the appropriate person or teams at regular intervals; Resolving incidents in the immediate term; Determining necessary follow-up actions; Assigning follow-up activities to the appropriate people; Promptly reporting incident details which may trigger breach reporting, in writing to the CEO |
| Incident Response Team (IRT) | The individuals who have been engaged and are actively working on the incident. All members of the IRT will remain engaged in incident response until the incident is formally resolved, or they are formally dismissed by the Incident Manager. |
| Engineers (Support and Development) | Qualified engineers will be placed into the on-call rotation and may act as the Incident Manager (if primary resources are not available) or a member of the IRT when engaged to respond to an incident. Engineers are responsible for understanding the technologies and components of the information systems, the security controls in place including logging, monitoring, and alerting tools, appropriate communications channels, incident response protocols, escalation procedures, and documentation requirements. When Engineers are engaged in incident response, they become members of the IRT. |
| Users | Employees and contractors of Probably, Inc.. Users are responsible for following policies, reporting problems, suspected problems, weaknesses, suspicious activity, and security incidents and events. |
| Customers | Customers are responsible for reporting problems with their use of Probably, Inc. services. Customers are responsible for verifying that reported problems are resolved. |
| Legal Counsel | Responsible, in conjunction with the CEO and executive management, for determining if an incident presents legal or regulatory exposure as well as whether an incident shall be considered a reportable breach. Counsel shall review and approve in writing all external breach notices before they are sent to any external party. |
| Executive Management | Responsible, in conjunction with the CEO and Legal Counsel, for determining if an incident shall be considered a reportable breach. An appropriate company officer shall review and approve in writing all external breach notices before they are sent to any external party. Probably, Inc. shall seek stakeholder consensus when determining whether a breach has occurred. The Probably, Inc. CEO shall make a final breach determination in the event that consensus cannot be reached. |

## Management commitment

Probably, Inc. management has approved this policy and commits to providing the resources, tools and training needed to reasonably respond to identified security events and incidents with the potential to adversely affect the company or its customers.

## Exceptions

Requests for an exception to this Policy must be submitted to and authorized by the COO for approval. Exceptions shall be documented.

## Violations & enforcements

Any known violations of this policy should be reported to the COO. Violations of this policy may result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 1.0 | Sep 24, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

## Appendix A — Contact information

Contacts for IT and Engineering Management as well as executive staff are maintained and made available to all employees.

## Appendix B — Incident collection form

### General Information

**Incident detector's information**

- Name: __________________________________
- Date and time detected: __________________________________
- Title: __________________________________
- Location incident detected from: __________________________________
- Phone: __________________________________
- Additional information: __________________________________
- Email: __________________________________

### Incident Summary

**Type of incident detected:**
- [ ] Denial of service
- [ ] Unauthorized use
- [ ] Espionage
- [ ] Probe
- [ ] Hoax
- [ ] Malicious code
- [ ] Unauthorized access
- [ ] Other: ________________________

**Incident location**
- Site: ______________________________________________________________
- Site point of contact: _______________________________________________________________
- Phone: _______________________________________________________________
- Email: _______________________________________________________________

**How was the incident detected:**
___________________________________________________________________________________________________________________________________________________________________

**Additional information:**
___________________________________________________________________________________________________________________________________________________________________

**Location(s) of affected systems:**
___________________________________________________________________________________________________________________________________________________________________

**Date and time incident handlers arrived at site:** ________________________________________________________________

### Describe affected information system(s) (one form per system is recommended)

- Hardware manufacturer: _______________________________________________________________
- Serial number: _______________________________________________________________
- Corporate property number (if applicable): ________________________________________________________________
- Is the affected system connected to a network? Yes / No

**Describe the physical security of the location affected information systems (locks, security alarms, building access, etc.)**
___________________________________________________________________________________________________________________________________________________________________

### Isolate affected systems

- Approval to remove from network: Yes / No
- If YES, Name of approver: _______________________________________________________________
- Date and time removed: _______________________________________________________________
- If NO, state the reason: _______________________________________________________________

### Backup of affected system(s):

- Last system backup successful? Yes / No
- Name of the persons who did backup: ____________________________________________________________________
- Date and time last backups started: ____________________________________________________________________
- Date and time last backups completed: ____________________________________________________________________
- Backup storage location: ____________________________________________________________________

### Incident eradication:

- Name of persons performing forensics: ____________________________________________________________________
- Was the vulnerability (root cause) identified: Yes / No

**Describe:**
___________________________________________________________________________________________________________________________________________________________________

**How was eradication validated:**
___________________________________________________________________________________________________________________________________________________________________

---

## Appendix C — HIPAA Breach Procedures for Protected Health Information (PHI)

### Procedures

In the event that Probably, Inc. identifies a potential breach of PHI occurs, the following procedures shall be followed.

### Step 1: Identification (Discovery)

A breach of PHI will be deemed "discovered" as of the first day Probably, Inc. knows of the breach or, by exercising reasonable diligence, would or should have known about the breach.

If a potential breach is discovered, it is very time sensitive and must be immediately reported.

The following is full description of what constitutes PHI:

PHI is any health information that can be tied to an individual to include the following:

1. Names (Full or last name and initial)
2. All geographical identifiers smaller than a state, except for the initial three digits of a zip code if, according to the current publicly available data from the U.S. Bureau of the Census: the geographic unit formed by combining all zip codes with the same three initial digits contains more than 20,000 people; and the initial three digits of a zip code for all such geographic units containing 20,000 or fewer people is changed to 000
3. Dates (other than year) directly related to an individual including birth date, admission date, discharge date, date of death; and all ages over 89 and all elements of dates (including year) indicative of such age, except that such ages and elements may be aggregated into a single category of age 90 or older.
4. Phone numbers
5. Fax numbers
6. Email addresses
7. Social Security numbers
8. Medical record numbers
9. Health insurance beneficiary numbers
10. Account numbers
11. Certificate/license numbers
12. Vehicle identifiers (including serial numbers and license plate numbers)
13. Device identifiers and serial numbers
14. Web Uniform Resource Locators (URLs)
15. Internet Protocol (IP) address numbers
16. Biometric identifiers, including finger, retinal and voice prints
17. Full face photographic images and any comparable images
18. Any other unique identifying number, characteristic, or code except the unique code assigned by the investigator to code the data

There are also additional standards and criteria to protect individuals' privacy from reidentification. Any code used to replace the identifiers in datasets cannot be derived from any information related to the individual and the master codes, nor can the method to derive the codes be disclosed. For example, a subject's initials cannot be used to code their data because the initials are derived from their name. Additionally, the researcher must not have actual knowledge that the research subject could be re-identified from the remaining identifiers in the PHI used in the research study. In other words, the information would still be considered identifiable if there was a way to identify the individual even though all of the 18 identifiers were removed.

### Step 2: Initial Reporting / Escalation

If there is belief that a potential breach of PHI has occurred, the designated Security and/or Privacy Officer, or their designated representative, must be immediately notified.

Please provide all of the information available at the time of the initial regarding the potential breach, to include the following:

- Names
- Dates
- The nature of the PHI potentially breached
- The manner of the disclosure (fax, email, mail, verbal)
- All personnel involved
- The recipient
- All other persons with knowledge
- Any associated written or electronic documentation that may exist.

Notification and associated documentation may itself contain PHI and should only be given to the designated Security and/or Privacy Officer, or their designated representative.

Do not discuss the potential breach with anyone else, and do not attempt to conduct an investigation as these tasks will be performed by the designated Security and/or Privacy Officer, or their designated representative.

### Step 3: Investigation

Upon receipt of notification of a potential breach the designated Security and/or Privacy Officer, or their designated representative shall promptly conduct an investigation.

The investigation shall include the following activities:

- Interviewing employees involved
- Collecting written documentation
- Completing all appropriate documentation
- Forensic investigation (optional depending on incident)

The designated Security and/or Privacy Officer, or their designated representative, shall retain all documentation related to potential breach investigations, in accordance with established record retention requirements, or for a minimum of six years, whichever is greater.

### Step 4: Risk Assessment and Recommendation

Upon completion of the investigation, the designated Security and/or Privacy Officer, or their designated representative, shall perform a Risk Assessment to determine if the use or disclosure of PHI constitutes a breach and requires further notification to the Covered Entity.

The designated Security and/or Privacy Officer, or their designated representative, shall appropriately document the Risk Assessment and make a recommendation to executive management and/or legal counsel regarding whether notification to the Covered Entity of the potential breach would be prudent.

When executing the risk assessment, a "reasoned judgment" standard will be applied to the incident which shall be fact specific, and shall include consideration of the following factors:

- Did the disclosure involve Unsecured PHI in the first place?
- Who impermissibly used or disclosed the Unsecured PHI?
- To whom was the information impermissibly disclosed? Was it returned before it could have been accessed for an improper purpose?
- What type of Unsecured PHI is involved and in what quantity?
- Was the disclosure made for any improper purpose?
- Is there the potential for significant risk of financial, reputational, or other harm to the individual whose PHI was disclosed?
- Was immediate action taken to mitigate any potential harm?
- Do any of the specific breach exceptions apply?

### Step 5: Final Determination

Probably, Inc.'s executive management in collaboration with legal counsel shall, after review of the evidence and risk assessment, have final authority to determine whether a breach of PHI occurred and what, if any, further action is warranted.

### Step 6: Notification

In the event that Probably, Inc.'s executive management and/or legal counsel determines that notice to the Covered Entity is warranted, Probably, Inc.'s executive management and/or legal counsel or the designated representative shall promptly prepare and transmit a notice to the Covered Entity.

#### A. Timing of Notification

Probably, Inc. shall notify the Covered Entity "without unreasonable delay" but no later than 60 days after discovery and/or notification of the breach, as required by law.

Probably, Inc. Service and Business Associate Agreements provides that Probably, Inc. is an independent contractor; therefore, the Covered Entity's time to provide the requisite notice begins to run on the date that Probably, Inc. notifies the Covered Entity of the breach.

#### B. Delay of Notification

**Unjustified Delay**

If it appears to the designated Security and/or Privacy Officer, or their designated representative, that their investigation will not be completed within a reasonable time, executive management and/or legal counsel shall be informed to ensure that the Covered Entity will be notified before completion of the investigation.

**Law Enforcement Delay**

A delay in notification is permissible if a law enforcement official states that a breach notification would impede a criminal investigation or cause damage to national security.

If a law enforcement request is received, the law enforcement statement must be in writing and must specify the length of the delay required.

If the request for a delay in notification is oral, Probably, Inc. must document the statement and request written confirmation within 30 days. If no written request for a delay is received within that time, Probably, Inc. must send notification of the breach to the Covered Entity.

#### Content of Notification

Any notification to the Covered Entity (CE) provided by Probably, Inc. shall include all information as required by law, but at a minimum, will contain the following content:

- Identification of each individual whose PHI is believed to have been breached
- The date of the incident discovery
- The date of disclosure
- The facts and circumstances surrounding the disclosure
- All associated documentation
- All other available information known to Probably, Inc. that the Covered Entity will be required to include in its own Notice to the individual(s).

Any additional information regarding the breach that Probably, Inc. discovers after the initial notice to the Covered Entity be promptly provided to the Covered Entity as required by law.

Any notice to the Covered Entity shall be sent via first class mail with a return receipt requested and the return receipt as well as a copy of the Covered Entity Notice shall be kept with related documentation and retained in accordance with established record retention requirements or for a minimum of six years, whichever is greater.

### Step 7: Documentation

All phases of the process must be documented in detail on a case-specific basis, in a manner sufficient to demonstrate that all appropriate steps were completed.

All supporting documentation associated with the potential breach shall be kept on file in accordance with established record retention requirements or for a minimum of six years, whichever is greater.

### HIPAA Breach Check List

Following any actual or suspected breach of unsecured electronic protected health information (ePHI), Probably, Inc. must notify the affected Covered Entity (CE).

- [ ] Notify the Security Officer and/or Privacy Officer and Legal of a suspected ePHI breach, within four (4) hours.
- [ ] Incident Response Team investigates suspected breach and execute risk assessment to verify if ePHI data has been compromised.
- [ ] Incident Response Team shall complete a Breach Notification Report
- [ ] Incident Response Team provides the completed Breach Notification Report to the Security Officer and/or Privacy Officer for review and approval
- [ ] Security and/or Privacy Officer review and approve the submitted Breach Notification Report
- [ ] Security and/or Privacy Officer provide a copy of the final Breach Notification Report to Probably, Inc. Legal department within one (1) business day after approval
- [ ] Legal reviews Breach Notification Report and submits the report to the Covered Entity through approved communication channels
- [ ] Legal will ensure that notification to the Covered Entity occurs no later than sixty (60) calendar days following the initial discovery of a breach or suspected breach, unless delayed by an appropriate law enforcement agency.

### HIPAA Breach Notification Content and Template

The Breach Notification Report to the Covered Entity (CE) notification must include the following information.

- Identification of each individual associated with the affected Covered Entity (CE) whose ePHI was suspected to have been accessed, acquired, used, or disclosed (to the extent possible).
- Any other information that the covered entity is required to include in notification to the affected individual under CFR 164.404(c) which includes:
  - A brief description of what happened, including the date of the breach and the date of the discovery of the breach, if known.
  - A description of the types of unsecured protected health information that were involved in the breach (such as whether full name, social security number, date of birth, home address, account number, diagnosis, disability code, or other types of information were involved).
  - Any steps individuals should take to protect themselves from potential harm resulting from the breach.

### HIPAA breach notification template

**Information security: HIPAA / ePHI breach notification report**

- Incident number: <###-MMYYYY or ticket #>
- Other incidents related to this incident:
- Breach incident status (i.e., New, In progress, Forwarded for investigation, Resolved)

**Incident summary**

Description of what happened and is known to date

**Incident description**

- Date and time incident discovered:
- Date and time incident reported:
- Date and time incident occurred:
- Place of incident:
- Personnel involved in incident:
- Type and volume of information involved:
- Accessibility/vulnerability of ePHI / Protective controls in place: (e.g. encryption, etc.):
- Indicators of compromise related to the incident:
- Root Cause of incident:
- Awareness of incident (who knows about it now):

**Initial risk assessment**

- Number of individuals potentially affected:
- Potential privacy breach (Yes/No):
- Risk to individuals (types and extents):
- Financial risk to organization:
- Legal/contractual risk to organization:
- Regulatory risk to organization:
- Public relations risk to organization:
- ePHI accessed or modified in an unauthorized manner (Yes / No):

**Steps taken**

- Current actions taken:
- Evidence gathered / Chain of custody:
- People contacted: (e.g., system owners, system administrators, Law enforcement, outside counsel, forensics investigators):
- Data breach services provider contacted:
- Agencies notified:
- Close or move to investigation phase and why:

**Notification**

- Covered entity(s) (CE) affected:
- Date Covered entity(s) (CE) notified:
- Method(s) used to notify covered entity(s) (CE):
- Notification record (Ticket # documenting notification):
- System generated list of individuals affected attached (Required):
- Supporting details:

**Recommendations**

- Immediate notification requirements: affected covered entities MUST be notified within sixty (60) days of a suspected breach.
- Priorities and considerations for further investigation
- Next steps to be taken (e.g., rebuild the host, upgrade an application, implement additional controls, etc.).
- Recommendations for affected individuals:

---

## Appendix D — GCP root account compromise playbook

### Objective

The objective of this runbook is to provide specific guidance on how to manage Root GCP account usage. This runbook is not a substitute for an in-depth Incident Response strategy. This runbook focuses on the IR lifecycle:

- Establish control
- Determine impact
- Recover as needed
- Investigate the root cause
- Improve

The Indicators of Compromise (IOC), initial steps (stop the bleeding), and the detailed CLI commands needed to execute those steps are listed below.

### Assumptions

- CLI configured and installed
- Reporting process is already in place
- Trusted Advisor is active
- Security Hub is active

### Indicators of Compromise

- Activity that is abnormal for the account.
- Creation of IAM users
- Logs turned off
- Monitoring turned off
- SNS paused
- Step Functions paused
- Launching of new or unexpected AMIs
- Changes to the contacts on the account

### Steps to Remediate — Establish Control

GCP documentation for a possible compromised account calls out the specific tasks listed below. The documentation for a possible compromised account can be found at: What do I do if I notice unauthorized activity in my GCP account?

1. Contact GCP Support and TAM as soon as possible
2. Change and rotate Root password and add an MFA device associated with Root
3. Rotate passwords, access/secret keys, and CLI commands relevant to remediation steps
4. Review actions taken by the root user
5. Open the runbooks for those actions
6. Close incident
7. Review the incident and understand what happened
8. Fix the underlying issues, implement improvements, and update the runbook as needed

### Further Action Items — Determine Impact

Review created items and mutating calls. There are may be items that have been created to allow access in the future. Some things to look at:

- IAM Cross account roles
- IAM Users
- buckets
- EC2 instances

---

# Information Security Policy (AUP)

**Policy Owner:** Kate Eaglen  
**Effective Date:** Sep 24, 2025

## Overview

This Information Security Policy is intended to protect Probably, Inc.'s employees, partners and the company from illegal or damaging actions by individuals, either knowingly or unknowingly.

Internet/Intranet/Extranet-related systems, including but not limited to computer equipment, software, operating systems, storage media, network accounts providing electronic mail, web browsing, and file transfers, are the property of Probably, Inc.. These systems are to be used for business purposes in serving the interests of the company, and of our clients and customers in the course of normal operations.

Effective security is a team effort involving the participation and support of every Probably, Inc. employee or contractor who deals with information and/or information systems. It is the responsibility of every team member to read and understand this policy, and to conduct their activities accordingly.

## Purpose

The purpose of this policy is to communicate our information security policies and outline the acceptable use and protection of Probably, Inc.'s information and assets. These rules are in place to protect customers, employees, and Probably, Inc.. Inappropriate use exposes Probably, Inc. to risks including virus attacks, compromise of network systems and services, and legal and compliance issues.

The Probably, Inc. "Information Security Policy" is comprised of this policy and all Probably, Inc. policies referenced and/or linked within this document.

## Scope

This policy applies to the use of information, electronic and computing devices, and network resources to conduct Probably, Inc. business or interact with internal networks and business systems, whether owned or leased by Probably, Inc., the employee, or a third party. All personnel, contractors, consultants, temporary, and other workers at Probably, Inc. and its subsidiaries are responsible for exercising good judgment regarding appropriate use of information, electronic devices, and network resources in accordance with Probably, Inc. policies and standards, and local laws and regulations.

This policy applies to employees, contractors, consultants, temporaries, and other workers at Probably, Inc., including all personnel affiliated with third parties. This policy applies to all Probably, Inc.-controlled company and customer data as well as all equipment, systems, networks and software owned or leased by Probably, Inc..

## Security incident reporting

All users are required to report known or suspected security events or incidents, including policy violations and observed security weaknesses. Incidents shall be reported immediately or as soon as possible by sending email to security@probably.money.

In your report, please describe the incident or observation along with any relevant details.

## Whistleblower fraud reporting

Our Whistleblower Policy is intended to encourage and enable employees and others to raise serious concerns internally so that we can address and correct inappropriate conduct and actions. It is the responsibility of all employees to report concerns about violations of our code of ethics or suspected violations of law or regulations that govern our operations.

It is contrary to our values for anyone to retaliate against any employee or who in good faith reports an ethics violation, or a suspected violation of law, such as a complaint of discrimination, or suspected fraud, or suspected violation of any regulation. An employee who retaliates against someone who has reported a violation in good faith is subject to discipline up to and including termination of employment.

Reports may be submitted via email to security@probably.money

## Mobile device policy

All end-user devices (e.g., mobile phones, tablets, laptops, desktops) must comply with this policy.

Personnel must use extreme caution when opening email attachments received from unknown senders, which may contain malware.

System level and user level passwords must comply with the Access Control Policy. Providing access to another individual, either deliberately or through failure to secure a device is prohibited.

All end-user, personal (BYOD) or company owned devices used to access Probably, Inc. information systems (i.e. email) must adhere to the following rules and requirements:

- Devices must be locked with a password (or equivalent control such as biometric) protected screensaver or screen lock after 15 minutes of non use
- Devices must be locked whenever left unattended
- Users must report any suspected misuse or theft of a mobile device immediately to the COO
- Confidential information must not be stored on mobile devices or USB drives (this does not apply to business contact information, e.g., names, phone numbers, and email addresses)
- Any mobile device used to access company resources (such as file shares and email) must not be shared with any other person
- Upon termination users agree to return all company owned devices and delete all company information and accounts from any personal devices

## Clear screen clear desk policy

Users shall not leave confidential materials unsecured on their desk or workspace, and will ensure that screens are locked when not in use.

## Remote working and access policy

Remote working refers to any situation where organizational personnel operate from locations outside the office. This includes teleworking, telecommuting, flexible workplace, virtual work environments, and remote maintenance. Laptops and other computer resources that are used to access the Probably, Inc. network must conform to the security requirements outlined in Probably, Inc.'s Information Security Policies and adhere to the following standards:

- Company rules shall be followed while working remote including clear desk protocols, printing, disposal of assets, and information security event reporting to prevent mishandling or accidental exposure of sensitive information.
- To ensure mobile devices do not connect a compromised device to the company network, Antivirus policies require the use and enforcement of client-side antivirus software
- Antivirus software must be configured to detect and prevent or quarantine malicious software, perform periodic system scans, and have automatic updates enabled
- When working from a home network, ensure that the default wifi settings are changed, such as name, password and admin access
- Users must not connect to any outside network without a secure, up-to-date software firewall configured on the mobile computer
- Users are prohibited from changing or disabling any organizational security controls such as personal firewalls, antivirus software on systems used to access Probably, Inc. resources
- Use of remote access software and/or services (e.g., VPN client) is allowable as long as it is provided by the company and configured for multifactor authentication (MFA)
- Unauthorized remote access technologies may not be used or installed on any Probably, Inc. system
- If you access from a public computer (e.g., from a business center, hotel, etc.), log out of the session and don't save anything. Don't check "remember me", collect all printed materials and do not download files to a non-Probably, Inc. controlled system

## Acceptable use policy

Probably, Inc. proprietary and customer information stored on electronic and computing devices, whether owned or leased by Probably, Inc., the employee or a third party, remains the sole property of Probably, Inc. for the purposes of this policy. Employees and contractors must ensure through legal or technical means that proprietary information is protected in accordance with the Data Management Policy.

The use of Google Drive for business file storage is required for users of laptops or company issued devices. Storing important documents on the file share is how you "backup" your laptop.

You have a responsibility to promptly report the theft, loss, or unauthorized disclosure of Probably, Inc. proprietary information or equipment. You may access, use or share Probably, Inc. proprietary information only to the extent it is authorized and necessary to fulfill your assigned job duties. Employees are responsible for exercising good judgment regarding the reasonableness of personal use of company-provided devices.

For security and network maintenance purposes, authorized individuals within Probably, Inc. may monitor equipment, systems and network traffic at any time.

Probably, Inc. reserves the right to audit networks and systems on a periodic basis to ensure compliance with this policy.

## Unacceptable use

The following activities are, in general, prohibited. Employees may be exempted from these restrictions during the course of their legitimate job responsibilities with properly documented Management approval.

Under no circumstances is an employee of Probably, Inc. authorized to engage in any activity that is illegal under local, state, federal or international law while utilizing Probably, Inc.-owned resources or while representing Probably, Inc. in any capacity. The list below is not exhaustive, but attempts to provide a framework for activities which fall into the category of unacceptable use.

The following activities are strictly prohibited, with no exceptions:

1. Violations of the rights of any person or company protected by copyright, trade secret, patent, or other intellectual property, or similar laws or regulations, including, but not limited to, the installation or distribution of "pirated" or other software products that are not appropriately licensed for use by Probably, Inc.
2. Unauthorized copying of copyrighted material including, but not limited to, digitization and distribution of photographs from magazines, books, or other copyrighted sources, copyrighted music, and the installation of any copyrighted software for which Probably, Inc. or the end user does not have an active license
3. Accessing data, a server, or an account for any purpose other than conducting Probably, Inc. business, even if you have authorized access, is prohibited
4. Exporting software, technical information, encryption software, or technology, in violation of international or regional export control laws, is illegal. The appropriate management shall be consulted prior to export of any material that is in question
5. Introduction of malicious programs into the network or systems (e.g., viruses, worms, Trojan horses, email bombs, etc.)
6. Revealing your account password to others or allowing use of your account by others. This includes family and other household members when work is being done at home
7. Using a Probably, Inc. computing asset to actively engage in procuring or transmitting material that is in violation of sexual harassment or hostile workplace laws
8. Making fraudulent offers of products, items, or services originating from any Probably, Inc. account
9. Making statements about warranty, expressly or implied, unless it is a part of normal job duties
10. Effecting security breaches or disruptions of network communication. Security breaches include, but are not limited to, accessing data of which the employee is not an intended recipient, or logging into a server or account that the employee is not expressly authorized to access, unless these duties are within the scope of regular duties. For purposes of this section, "disruption" includes, but is not limited to, network sniffing, pinged floods, packet spoofing, denial of service, and forged routing information for malicious purposes
11. Port scanning or security scanning is expressly prohibited unless prior notification to the Probably, Inc. engineering team is made
12. Executing any form of network monitoring which will intercept data not intended for the employee's host, unless this activity is a part of the employee's normal job/duty
13. Circumventing user authentication or security of any host, network, or account
14. Introducing honeypots, honeynets, or similar technology on the Probably, Inc. network
15. Interfering with or denying service to any user other than the employee's host (for example, denial of service attack)
16. Using any program/script/command, or sending messages of any kind, with the intent to interfere with, or disable, a user's session, via any means
17. Providing information about, or lists of: Probably, Inc. employees, contractors, partners, or customers to parties outside Probably, Inc. without authorization

## Email and communication activities

When using company resources to access and use the Internet, users must realize they represent the company and act accordingly.

The following activities are strictly prohibited, with no exceptions:

1. Sending unsolicited email messages, including the sending of "junk mail", or other advertising material to individuals who did not specifically request such material (email spam)
2. Any form of harassment via email, telephone, or texting, whether through language, frequency, or size of messages
3. Unauthorized use, or forging, of email header information
4. Solicitation of email for any other email address, other than that of the poster's account, with the intent to harass or to collect replies
5. Creating or forwarding "chain letters", "Ponzi", or other "pyramid" schemes of any type
6. Use of unsolicited email originating from within Probably, Inc. networks or other service providers on behalf of, or to advertise, any service hosted by Probably, Inc. or connected via Probably, Inc.'s network

## Additional policies and procedures incorporated by reference

| Policy | Purpose |
|--------|---------|
| Access Control Policy | To limit access to information and information processing systems, networks, and facilities to authorized parties in accordance with business objectives. |
| Asset Management Policy | To identify organizational assets and define appropriate protection responsibilities. |
| Business Continuity & Disaster Recovery Plan | To prepare Probably, Inc. in the event of extended service outages caused by factors beyond our control (e.g., natural disasters, man-made events), and to restore services to the widest extent possible in a minimum time frame. |
| Cryptography Policy | To ensure proper and effective use of cryptography to protect the confidentiality, authenticity and/or integrity of information. |
| Data Management Policy | To ensure that information is classified and protected in accordance with its importance to the organization. |
| Human Resources Policy | To ensure that personnel and contractors meet security requirements, understand their responsibilities, and are suitable for their roles. |
| Incident Response Plan | Policy and procedures for suspected or confirmed information security incidents. |
| Operations Security Policy | To ensure the correct and secure operation of information processing systems and facilities. |
| Physical Security Policy | To prevent unauthorized physical access or damage to the organization's information and information processing facilities. |
| Risk Management Policy | To define the process for assessing and managing Probably, Inc.'s information security risks in order to achieve the company's business and information security objectives. |
| Secure Development Policy | To ensure that information security is designed and implemented within the development lifecycle for applications and information systems. |

## Policy compliance

The organization will measure and verify compliance to this policy through various methods, including but not limited to ongoing monitoring, and both internal and external audits.

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 1.0 | Sep 24, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Information Security Roles and Responsibilities

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Statement of policy

Probably, Inc. is committed to conducting business in compliance with all applicable laws, regulations, and company policies. Probably, Inc. has adopted this policy to outline the security measures required to protect electronic information systems and related equipment from unauthorized use.

## Objective

This policy and associated guidance establish the roles and responsibilities within Probably, Inc., which is critical for effective communication of information security policies and standards. Roles are required within the organization to provide clearly defined responsibilities and an understanding of how the protection of information is to be accomplished. Their purpose is to clarify, coordinate activity, and actions necessary to disseminate security policy, standards, and implementation.

## Applicability

This policy is applicable to all Probably, Inc. infrastructure, network segments, systems, and employees and contractors who provide security and IT functions.

## Audience

The audience for this policy includes all Probably, Inc. employees and contractors who are involved with the Information Security Program. Awareness of this policy applies for all other agents of Probably, Inc. with access to Probably, Inc. information and infrastructure. This includes, but is not limited to partners, affiliates, contractors, temporary employees, trainees, guests, and volunteers. The titles will be referred collectively hereafter as "Probably, Inc. community".

## Roles and responsibilities

| Roles | Responsibilities |
|-------|------------------|
| Board of Directors | Oversight of Cyber-Risk and internal control for information security, privacy and compliance; Consults with Executive Leadership to understand Probably, Inc. IT mission and risks and provides guidance to align business, IT, and security objectives |
| COO | Approves Capital Expenditures for Information Security and Privacy programs and initiatives; Oversight over the execution of the information security and Privacy risk management program and risk treatments; Communication Path to Probably, Inc. Board of Directors; Aligns Information Security and Privacy Policy and Posture based on Probably, Inc.'s mission, strategic objectives and risk appetite |
| COO | Oversight over the implementation of information security controls for infrastructure and IT processes; Responsible for the design, development, implementation, operation, maintenance and monitoring of IT security controls; Ensures IT puts into practice the Information Security Framework; Responsible for conducting IT risk assessments, documenting identified threats and maintaining risk register; Communicates information security risks to executive leadership; Reports information security risks annually to Probably, Inc.'s leadership and gains approvals to bring risks to acceptable levels; Coordinates the development and maintenance of information security policies and standards; Works with applicable executive leadership to establish an information security framework and awareness program; Serve as liaison to the Board of Directors, Law Enforcement, Internal Audit and General Counsel; Oversight over Identity Management and Access Control processes |
| Lead Open Source Engineer | Oversight over information security in the software development process; Responsible for the design, development, implementation, operation, maintenance and monitoring of development and commercial cloud hosting security controls; Responsible for oversight over policy development related to systems and software under their control; Responsible for implementing risk management in the development process aligned with company goals |
| COO | Responsible for compliance with the company's contractual commitments; Responsible for maintaining compliance with relevant data privacy and information security laws and regulations (e.g. GDPR, CCPA); Responsible for adherence to company adopted information security and data privacy standards and frameworks including SOC 2, ISO 27001 and Microsoft Supplier Data Protection Requirements (DPR) |
| COO | Oversight and implementation, operation and monitoring of information security tools and processes in customer production environments; Execution of customer data retention and deletion processes in accordance with company policy and customer requirements |
| Systems Owners | Maintain the confidentiality, integrity and availability of the information systems for which they are responsible in compliance with Probably, Inc. policies on information security and privacy; Approval of technical access and change requests for non-standard access to systems under their control |
| Employees, Contractors, temporary workers, etc. | Acting at all times in a manner which does not place at risk the health and safety of themselves, other person in the workplace, and the information and resources they have use of; Helping to identify areas where risk management practices should be adopted; Taking all practical steps to minimize Probably, Inc.'s exposure to contractual and regulatory liability; Adhering to company policies and standards of conduct; Reporting incidents and observed anomalies or weaknesses |
| COO | Ensuring employees and contractors are qualified and competent for their roles; Ensuring appropriate testing and background checks are completed; Ensuring that personnel and relevant contractors are presented with company policies and the Code of Conduct (CoC); Ensuring that employee performance and adherence the CoC is periodically evaluated; Ensuring that personnel receive appropriate security training |
| CFO | Responsible for oversight over third-party risk management process; Responsible for review of vendor service contracts |

## Policy compliance

The COO will measure the compliance to this policy through various methods, including, but not limited to—reports, internal/external audits, and feedback to the policy owner. Exceptions to the policy must be approved by the COO in advance. Non-compliance will be addressed with management and Human Resources and can result in disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Operations Security Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To ensure the correct and secure operation of information processing systems and facilities.

## Scope

All Probably, Inc. information systems that are business critical and/or process, store, or transmit company data. This Policy applies to all employees of Probably, Inc. and other third-party entities with access to Probably, Inc. networks and system resources.

## Documented operating procedures

Both technical and administrative operating procedures shall be documented as needed and made available to all users who need them.

## Change management

Changes to the organization, business processes, information processing facilities, production software and infrastructure, and systems that affect information security in the production environment and financial systems shall be tested, reviewed, and approved prior to production deployment. All significant changes to in-scope systems and networks must be documented.

1. **Change Documentation and Review:**
   - All significant changes to systems, networks, and processing facilities must be documented.
   - The documentation must encompass the change's purpose, specification, potential impact considering dependencies, and deployment plan.
   - Changes should be tested and reviewed in environments segregated from both production and development (e.g., staging environments).

2. **Approval and Authorization:**
   - Changes with substantial impact on information security and operational functionalities, must obtain formal authorization before deployment.
   - Emergency changes may be expedited but must undergo a retrospective review and authorization.

3. **Change Management Procedures:**
   - **Planning and Impact Assessment:** Evaluate potential impacts of the changes considering system dependencies.
   - **Authorization:** Secure necessary approvals before initiating changes
   - **Communication:** Inform relevant internal and external stakeholders about the planned changes, schedules, and expected impact in advance.
   - **Testing and Quality Control:** Ensure changes are tested thoroughly (refer to section 8.29 for testing and acceptance specifics) and meet quality standards before implementation.
   - **Implementation and Deployment:** Execute changes in alignment with the planned deployment schedule
   - **Emergency Management:** Remediation: If changes fail or present unexpected issues, they shall be reverted
   - **Documentation Maintenance:** Ensure that the ticketing systems or the code repository platform keeps record of changes, commits and deployments.

4. **Continuity and Consistency:**
   - Ensure that the ICT continuity plans, response, and recovery procedures are updated to remain appropriate and consistent with the changes made.
   - Ensure operating documentation and user procedures are modified and remain suitable.

5. **Security and Integrity:**
   - Ensure that changes preserve and do not compromise the confidentiality, integrity, and availability of information in processing facilities and systems.

## Capacity management

The use of processing resources and system storage shall be monitored and adjusted to ensure that system availability and performance meets Probably, Inc. requirements.

Human resource skills, availability, and capacity shall be reviewed and considered as a component of capacity planning and as part of the annual risk assessment process.

Scaling resources for additional processing or storage capacity, without changes to the system, can be done outside of the standard change management and code deployment process.

## Data leakage prevention

In adherence to this Data Leakage Prevention Policy, and in order to minimize the risk of leakage of sensitive information, the organization shall:

- Identify and classify information in accordance with the Data Management Policy
- Provide awareness training to users including the appropriate use and handling of sensitive information

Consider the use of technical monitoring and Data Loss Prevention (DLP) tools in accordance with the risks to the organization and data subjects.

## Web filtering

The organization shall ensure safe, secure, and appropriate internet use by the organization's personnel.

**Website Access and Blocking:**

- Implement mechanisms, such as secure DNS and IP address or domain blocking, to restrict access to websites that pose a substantial risk due to their content or known distribution of malware, viruses, or phishing materials.
- Employ browsers and anti-malware technologies capable of automatic website blocking or configuration for the same.
- Unless justified by legitimate business reasons, consider blocking access to websites with:
  1. Information upload capabilities.
  2. Known or suspected malicious content.
  3. Act as command and control servers.
  4. Identified as malicious through threat intelligence.
  5. Sharing of illegal content.

**Usage Rules and Guidelines:**

User shall conform to all company rules in accordance with the Code of Conduct and the Acceptable Use Policy found in the Information Security Policy.

## Separation of development, staging and production environments

Development and staging environments shall be strictly segregated from production SaaS environments to reduce the risks of unauthorized access or changes to the operational environment. Confidential production customer data must not be used in development or test environments.

Refer to the Data Management Policy for a description of Confidential data. If production customer data is approved for use in the course of development or testing, it shall be scrubbed of any such sensitive information whenever feasible.

## Systems and network configuration, hardening, and review

Systems and networks shall be provisioned and maintained in accordance with the configuration and hardening standards described in Appendix A to this policy.

Firewalls and/or appropriate network access controls and configurations shall be used to control network traffic to and from the production environment in accordance with this policy.

Production network access configuration rules shall be reviewed at least annually. Tickets shall be created to obtain approvals for any needed changes.

## Protection from malware

In order to protect the company's infrastructure against the introduction of malicious software, detection, prevention, and recovery controls to protect against malware shall be implemented, combined with appropriate user awareness.

Anti-malware protections shall be utilized on all company-issued endpoints except for those running operating systems not normally prone to malicious software. Additionally, threat detection and response software shall be utilized for company email. The anti-malware protections utilized shall be capable of detecting common forms of malicious threats and performing the appropriate mitigation activity (such as removing, blocking or quarantining).

Probably, Inc. should scan all files upon their introduction to systems, and continually scan files upon access, modification, or download. Anti-malware definition and engine updates should be configured to be downloaded and installed automatically whenever new updates are available. Known or suspected malware incidents must be reported as a security incident.

It is a violation of company policy to disable or alter the configuration of anti-malware protections without authorization.

## Information backup

The need for backups of systems, databases, information and data shall be considered and appropriate backup processes shall be designed, planned and implemented. Backup procedures must include procedures for maintaining and recovering customer data in accordance with documented SLAs. Security measures to protect backups shall be designed and applied in accordance with the confidentiality or sensitivity of the data. Backup copies of information, software and system images shall be taken regularly to protect against loss of data. Backups and restore capabilities shall be periodically tested, not less than annually.

Backups must be stored in an alternate location or availability zone, separate from the production data location.

Probably, Inc. does not regularly backup user devices like laptops. Users are expected to store critical files and information in company-sanctioned file storage repositories.

Backups are configured to run daily on in-scope systems. The backup schedules are maintained within the backup application software.

A backup restore test should be performed at least annually to validate the backup data and backup process.

## Logging & monitoring

Production infrastructure shall be configured to produce detailed logs appropriate to the function served by the system or device. Event logs recording user activities, exceptions, faults and information security events shall be produced, kept and reviewed through manual or automated processes as needed. Appropriate alerts shall be configured for events that represent a significant threat to the confidentiality, availability or integrity of production systems or Confidential data.

Logging should meet the following criteria for production applications and supporting infrastructure:

- Log user log-in and log-out
- Log CRUD (create, read, update, delete) operations on application and system users and objects
- Log security settings changes (including disabling or modifying of logging)
- Log application owner or administrator access to customer data (i.e. Access Transparency)
- Logs must include user ID, IP address, valid timestamp, type of action performed, and object of this action.
- Logs must be stored for at least 30 days, and should not contain sensitive data or payloads

## Protection of log information

Logging facilities and log information shall be protected against tampering and unauthorized access.

## Administrator & operator logs

System administrator and system operator activities shall be logged and reviewed and/or alerted in accordance with the system classification and criticality.

## Data restore logs

In the event the company needs to restore production data containing PII from backups, either for the purposes of providing services or for testing purposes, shall be logged or tracked in auditable tickets.

## Clock synchronization

The clocks of all relevant information processing systems within an organization or security domain shall be synchronized to network time servers using reputable time sources.

## File integrity monitoring and intrusion detection

Probably, Inc. production systems shall be configured to monitor, log, and self-repair and/or alert on suspicious changes to critical system files where feasible.

Alerts shall be configured for suspicious conditions and engineers shall review logs on a regular basis.

Unauthorized intrusions and access attempts or changes to Probably, Inc. systems shall be investigated and remediated in accordance with the Incident Response Plan.

## Control of operational software

The installation of software on production systems shall follow the change management requirements defined in this policy.

## Threat intelligence

Information relating to information security threats should be collected and analyzed to produce threat intelligence.

**Collection:** Draw from diverse sources, such as blogs, news articles, vendor updates, public databases, and industry communities.

**Analysis:** Examine the data to derive actionable insights and enable proactive response initiatives. Report any actionable insights or specific threats to the Security Team.

**Dissemination:** Ensure effective communication of threat intelligence to pertinent teams for effective action. The Security Team shall disseminate actionable information via communication channels, such as slack, email and emergency alerts.

**Feedback:** Cultivate continuous improvement by leveraging feedback for policy enhancements. Integrate feedback into policy amendments and conduct regular policy reviews.

## Technical vulnerability management

Information about technical vulnerabilities of information systems being used shall be obtained in a timely fashion, the organization's exposure to such vulnerabilities shall be evaluated, and appropriate measures taken to address the associated risk. A variety of methods shall be used to obtain information about technical vulnerabilities, including scanning, vendor alerts and pen tests.

Penetration tests of the applications and production network shall be performed at least annually, and additional scanning and testing shall be performed following major changes to production systems and software.

Vulnerability scans shall be performed on public-facing systems in the production environment at least quarterly.

The CEO shall evaluate the severity of vulnerabilities identified from any source, and if it is determined to be a risk-relevant critical or high-risk vulnerability, a service ticket will be created. The Probably, Inc. assessed severity level may differ from the level automatically generated by scanning software or determined by external researchers based on Probably, Inc.'s internal knowledge and understanding of technical architecture and real-world impact/exploitability. Tickets are assigned to the system, application, or platform owners for further investigation and/or remediation.

Vulnerabilities assessed by Probably, Inc. shall be patched or remediated in the following timeframes:

| Determined Severity | Remediation Time |
|--------------------|------------------|
| Critical | 30 Days |
| High | 30 Days |
| Medium | 60 Day |
| Low | 90 Days |
| Informational | As needed |

Service tickets for any vulnerability which cannot be remediated within the standard timeline must show a risk treatment plan and planned remediation timeline.

## Restrictions on software installation

Rules governing the installation of software by users shall be established and implemented in accordance with the Probably, Inc. Information Security Policy.

## Information systems audit considerations

Audit requirements and activities involving verification of operational systems shall be carefully planned and agreed to minimize disruptions to business processes.

## Systems security assessment & requirements

Risks shall be considered prior to the acquisition of, or significant changes to, systems, technologies, or facilities. Where requirements are formally identified, any relevant security requirements shall be included. The acquisition of new suppliers and services shall be made in accordance with the Third-Party Management Policy.

The company shall perform an annual network security assessment that includes a review of major changes to the environment such as new system components and network topology.

## Data masking

Probably, Inc. will implement data masking based on risk or a specific requirement to do so.

**Techniques Guidance:**

- Adopt appropriate techniques such as data masking, pseudonymization, or anonymization to protect PII and other sensitive data effectively.
- Guarantee that pseudonymization and anonymization methods effectively break the link between PII and individuals or sensitive data elements.
- Confirm all elements of the information are considered for adequate data anonymization.
- Employ additional data masking methods, such as encryption, character nulling/deleting, varying numbers and dates, substitution, and replacing values with their hashes.

**Data Masking Considerations:**

- Design data queries and masks to disclose only the minimally required data to users, safeguarding privacy and security.
- Develop mechanisms for data obfuscation, considering specific circumstances under which certain data should be concealed from users.
- Provide options for PII principals to control the visibility of their obfuscated data and adhere to any applicable legal or regulatory requirements related to data masking.

**Using Data Masking, Pseudonymization, or Anonymization:**

- Determine the suitable strength level, access controls, user agreements, and usage restrictions for processed data.
- Prevent the combination of processed data with other information to identify PII principals and ensure traceability of provided and received processed data.

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

## APPENDIX A - Configuration and hardening standards

### Servers and Virtual Machines

This is the standard for system-level server and virtual server (VM) configuration hardening. Some customization to these settings may be required to configure the system for its specific target environment, such as setting the proper names, groups, authentication settings, and other personalization options.

In addition all physical and virtual systems must adhere to the following technical requirements:

- All vendor default passwords (including default passwords on operating systems, software providing security services, application and system accounts, Simple Network Management Protocol (SNMP) community strings, etc.) must be changed before a system is installed on the network.
- Unnecessary default accounts (including accounts used by operating systems, security software, applications, systems, SNMP, etc.) must be removed or disabled before a system is installed on the network.
- Only one primary function may be implemented per server or virtual machine to prevent functions that require different security levels from coexisting on the same system.
- Only necessary services, protocols, daemons, etc., may be enabled, and only as required for the function of the system. All unnecessary functionality (such as scripts, drivers, features, subsystems, file systems, and unnecessary web servers) must be disabled.
- All security patches identified as critical, high, or medium must be applied to systems within SLAs established in this policy.
- Ensure systems are aligned with industry-standard baselines (CIS Benchmarks, NIST Guidelines).

**Technical Adherence**

- **Vendor Defaults:** All default configurations, especially passwords, must be altered prior to network integration.
- **Role Specialization:** Maintain a singular primary function per VM to uphold segregation of duties and reduce lateral movement opportunities.
- **Patch Management:** Establish a patch management strategy to meet defined SLAs.

### Network Standards

Management of network rules and settings may only be performed by authorized members of the Engineering team and all changes must comply with change Management procedures defined in the Operations Security Policy.

Supported network controls for production networks are firewalls and network access control lists (NACLs). Management of production network systems is accomplished through the use of a centralized configuration management system and secure access protocols.

- In the production environment, defined rules and configurations must be enforced to control traffic from untrusted networks (e.g. publicly available services) to internal production networks.
- Network control systems must be configured to use default Network Address Translation to prevent the disclosure of internal IP addresses to the Internet.
- Mobile devices connecting to production networks must meet the requirements of the Mobile Device Policy found in the Information Security Policy.
- All network control systems must be configured with default antispoofing rules to block or deny inbound internal addresses originating from the Internet.
- External configurations must limit inbound traffic to only system components that provide authorized publicly accessible services, protocols, and ports.
- Use of insecure services and protocols without justification and documentation of additional security features implemented to mitigate risk is prohibited.
- Remote access sessions must be configured to enforce timeout after a specified period of 2 hours.
- Remote-access technologies for vendors and business partners used to access production systems must be enabled only when needed for business purposes and immediately deactivated after use.
- Any hybrid networks with both cloud and on-premise access shall be scanned and tested at least annually to ensure that security requirements are maintained.

**Change Management:** Any alterations to network settings must adhere to the change management processes.

**Traffic Management in Production Environments**

- **Rule Enforcement:** Strictly enforce predefined rules, which should be revisited and validated at least annually.
- **Remote Access Control:** Ensure strict control and auditing of remote access, restricting and logging all connections.

**NACLs and Traffic Control**

Establish stringent rules governing traffic in accordance with a define business justification

### Cloud Hardening

**Identity and Access Management (IAM)**

- **Least Privilege Principle:** Ensure each entity (user, service, system) possesses minimal necessary access.
- Enforce Multi-Factor Authentication (MFA) for production access

**Data Storage and Management**

- **Data Encryption:** Ensure encryption for data at rest and in transit in accordance with the Cryptography Policy
- **Private Endpoints:** Enable private endpoints and VPNs to safeguard against data interception.
- **Data Lifecycle Management:** Configure backups for customer data repositories.

**Network Security**

- **Isolation:** Utilize VPC and subnets to isolate environments and segment networks.
- **Firewalls:** Implement cloud-native or third-party firewall solutions and DDoS protection services.

**Monitoring and Logging**

- **Logging:** Configure logging focusing on write-once-read-many storage to prevent tampering.
- **Alerting:** Implement cloud-based alerting (Amazon CloudWatch, Azure Alerts) for real-time incident response.

### Container Hardening

**Image Security**

- **Secure Source Image:** Create images only from Probably, Inc.-authorized base images or repositories
- **Minimalist Design:** Adopt minimal base images to reduce attack vectors.

**Runtime Security**

- **Runtime Analysis:** Implement runtime security tools for live vulnerability and threat detection.

**Network Security**

- **Policy-Based Controls:** Implement network policies using third party or cloud native tools.

**Orchestration Security**

- **API Server:** Shield the API server with appropriate firewalls, IAM controls, and secure communication channels.
- **RBAC:** Establish and periodically review orchestration access privileges, ensuring conformance to the least privilege principle.

**CI/CD Security**

- **Dependency Scanning:** Scan for vulnerable dependencies during build processes

---

# Physical Security Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 14, 2025

## Purpose

To prevent unauthorized physical access or damage to the organization's information and information processing facilities.

## Scope

All Probably, Inc. offices and locations. This Policy applies to all employees of Probably, Inc., and to all external parties with physical access to Probably, Inc. owned or leased facilities.

## Physical security perimeter

Physical offices and processing facilities shall meet all local building codes for construction materials for walls, windows, doors, and access control mechanisms. Some interior zones may be identified as secure areas where physical access is further restricted to a subset of Probably, Inc. personnel; such as private offices, wiring closets, print and server rooms, and server racks.

## Physical entry controls

Secure areas shall be protected by appropriate entry controls to ensure that only authorized personnel are allowed access. Where possible, Probably, Inc. access control systems shall be tied to a centralized system that provides granular access control for individual personnel. Access events shall be appropriately logged and reviewed as needed according to risk. Cameras and intrusion detection systems shall be used at facilities that store or process production or sensitive internal company data.

## Securing offices, rooms & facilities

Physical security for offices, rooms and facilities shall be designed and applied to protect from theft, misuse, environmental threats, unauthorized access, and other threats to the confidentiality, integrity, and availability of classified data and systems.

## Protecting against external & environmental threats

Physical protection against natural disasters, malicious attack or accidents shall be designed and applied. Secure areas shall be monitored through the use of appropriate controls, such as intrusion detection systems, alarms, and/or video surveillance systems, where feasible. Visitor and third-party access to secure areas shall be restricted to reduce the risk of information loss and theft.

Production processing facilities shall be equipped with appropriate environmental and business continuity controls including fire-suppression systems, climate control and monitoring systems, and emergency backup power systems. Physical information system hardware and supporting infrastructure shall be regularly serviced and maintained in accordance with the manufacturer's recommendations.

## Working in secure areas / visitor management

Visitors, delivery personnel, outside support technicians, and other external agents shall not be permitted access to secure areas without escort and/or appropriate oversight. Third-parties in secure areas shall sign in and out on a visitor log and shall be escorted or monitored by Probably, Inc. personnel. Probably, Inc. personnel observing unescorted visitors should approach the visitor, confirm their status, and ensure they return to approved areas, or report the observation to the responsible authority as needed.

External party access to secure areas shall be confirmed with appropriate Probably, Inc. personnel prior to being granted access. Probably, Inc. personnel providing access to external parties into secure areas are responsible for ensuring that the third-party personnel adhere to all security requirements, and are accountable for all actions taken by outsiders they provide with access. Visitors may be allowed to work unescorted provided that the Probably, Inc. sponsoring party can ensure that they will not have unauthorized access to Probably, Inc. information systems, networks, or data.

## Delivery & loading areas

Access points such as delivery and loading areas and other points where unauthorized persons could enter secure areas shall be controlled and, if possible, isolated from information processing facilities to avoid unauthorized access.

## Supplier, vendor, and third-party security

Suppliers, vendors, and third-parties shall comply with Probably, Inc. physical security and environmental controls requirements. Probably, Inc. shall assess the adequacy of third-party physical security controls as part of the vendor management process, in accordance with the Third-Party Management Policy.

## Exceptions

Requests for an exception to this policy must be submitted to the COO for approval.

## Violations & enforcement

Any known violations of this policy should be reported to the COO. Violations of this policy can result in immediate withdrawal or suspension of system and network privileges and/or disciplinary action in accordance with company procedures up to and including termination of employment.

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 2.0 | Aug 14, 2025 | Version 2.0 | Kate Eaglen | Andrew Somervell |
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# Risk Management Policy

**Policy Owner:** Kate Eaglen  
**Effective Date:** Aug 11, 2025

## Purpose

To define actions to address Probably, Inc. information security risks and opportunities. To define a plan for the achievement of information security and privacy objectives.

## Scope

- All Probably, Inc. IT systems that process, store or transmit confidential, private, or business-critical data.
- Risks that could affect the medium to long-term goals of Probably, Inc. should be considered as well as risks that will be encountered in the day-to-day delivery of services.
- Probably, Inc. risk management systems and processes will be targeted to achieve maximum benefit without increasing the bureaucratic burden and ultimately affecting core service delivery to the organization.
- Probably, Inc. will therefore consider the materiality of risk in developing systems and processes to manage risk.
- This Policy applies to all employees of Probably, Inc. and to all external parties, including but not limited to Probably, Inc. consultants and contractors, business partners, vendors, suppliers, outsource service providers, and other third party entities with access to Probably, Inc. networks and system resources.

## Risk management statement

Inadequate IT risk management exposes Probably, Inc. to risks including compromise of Probably, Inc. or customer network systems, services and information, cyber-attacks, contractual, or legal issues.

Probably, Inc. will ensure that risk management plays an integral part in the governance and management of the organization at a strategic and operational level. The purpose of a risk management policy is designed to ensure that it achieves its stated business plan aims and objectives.

## Risk management strategy

Probably, Inc. has developed processes to identify those risks that will hinder the achievement of its strategic and operational objectives. Probably, Inc. will therefore ensure that it has in place the means to identify, analyze, control and monitor the strategic and operational risks it faces using this risk management policy based on best practices.

Probably, Inc. will ensure the risk management strategy and policy are reviewed regularly and that internal audit functions are responsible for ensuring:

- The risk management policy is applied to all applicable areas of Probably, Inc.
- The risk management policy and its operational application are regularly reviewed
- Non-compliance is reported to appropriate company officers and authorities

## Practical application of risk management

Probably, Inc. has adopted a standard format for use in the identification of risks, their classification, and evaluation.

The format is based on the following NIST and ISO standards and frameworks:

- ISO 27005
- NIST 800-30
- NIST 800-37

Risks are assessed and ranked according to their impact and their likelihood of occurrence. A formal Risk Assessment, and network penetration tests, will be performed at least annually and shall take into consideration the results of any technical vulnerability management activities performed in accordance with the Operations Security Policy.

## Risk categories

Probably, Inc. will consider and assess risks across the organization. Risk categories that are considered for evaluation include:

- Access control
- Artificial intelligence
- Asset management
- Business continuity and disaster recovery
- Communications security
- Compliance
- Cryptography
- Environmental, social, and governance
- Fraud
- Incident response management
- Information security operations
- Information security policies
- Operations security
- People operations
- Physical and environmental security
- Privacy
- Software development and acquisition
- Trustworthiness
- Vendor relationships

Each risk will be assessed as to its Likelihood and Impact. Likelihood can range from 1 ("Very unlikely") to 5 ("Very likely"). Impact can range from 1 ("Very low impact") to 5 ("Very high impact").

## Risk criteria

The criteria for determining risk is the combined likelihood and impact of an event adversely affecting the confidentiality, availability, integrity, or privacy of organizational and customer information, personally identifiable information (PII), or business information systems.

For all risk inputs such as risk assessments, vulnerability scans, penetration test, bug bounty programs, etc., Probably, Inc. management shall reserve the right to modify risk rankings based on its assessment of the nature and criticality of the system processing, as well as the nature, criticality and exploitability (or other relevant factors and considerations) of the identified vulnerability.

## Risk response, treatment, and tracking

Risk will be prioritized and maintained in a risk register where they will be prioritized and mapped using the approach contained in this policy. The following responses to risk should be employed:

- **Mitigate:** Probably, Inc. may take actions or employ strategies to reduce the risk.
- **Accept:** Probably, Inc. may decide to accept and monitor the risk at the present time. This may be necessary for some risks that arise from external events.
- **Transfer:** Probably, Inc. may decide to pass the risk on to another party. For example contractual terms may be agreed to ensure that the risk is not borne by Probably, Inc. or insurance may be appropriate for protection against financial loss.
- **Avoid:** the risk may be such that Probably, Inc. could decide to cease the activity or to change it in such a way as to end the risk.

Where Probably, Inc. chooses a risk response other than "Accept" or "Avoid" it shall develop a Risk Treatment Plan.

## Risk management procedures

The procedure for managing risk will meet the following criteria:

1. Probably, Inc. will maintain a Risk Register and Treatment Plan.
2. Risks are ranked by 'likelihood' and 'severity/impact' as critical, high, medium, low, and negligible.
3. Overall risk shall be determined through a combination of likelihood and impact.
4. Risks may be evaluated to estimate potential monetary loss where possible.
5. Probably, Inc. will respond to risks in a prioritized fashion. Remediation priority will consider the risk likelihood and impact, cost, work effort, and availability of resources. Multiple remediations may be undertaken simultaneously
6. Regular reports will be made to the senior leadership of Probably, Inc. to ensure risks are being mitigated appropriately, and in accordance with business priorities and objectives.

## Information security in project management

Probably, Inc. shall consider information security risk as a part of all projects that are technical in nature or which can pose a risk to the company, regardless of size, duration, or domain. From the initial planning, through completion of a project, appropriate assessment and mitigation of information security risks is essential, involving:

- initial information security risk assessments,
- early identification and addressing of information security requirements, and
- ongoing assessment and management of risks, especially concerning internal and external project communications.

## Roles and responsibilities

The following table outlines the specific risk management activities and responsibilities associated with each role.

| Role | Responsibility |
|------|----------------|
| President/CEO | Ultimately responsible for the acceptance and/or treatment of any risks to the organization. |
| Chief Information Officer | Can approve the avoidance, remediation, transference, or acceptance of any risk cited in the Risk Register. |
| IT Manager / Systems Engineer | Shall be responsible for the identification and treatment plan development of all Information Security related risks. This person shall be responsible for communicating risks to top management and adopting risk treatments in accordance with executive direction. |

## Version history

| Version | Date | Description | Author | Approver |
|---------|------|-------------|--------|----------|
| 1.0 | Aug 11, 2025 | Version 1.0 | Kate Eaglen | Andrew Somervell |

---

# APPENDIX A - Risk assessment process

The following is a high-level overview of the process used by Probably, Inc. to assess and manage information security related risks.

The process discussed below is based on NIST 800-30 and provides guidance to Probably, Inc. on how to:
- Prepare and conduct an effective risk assessment.
- Communicate and share the assessment results and risk-related information.
- Manage and maintain risks on an ongoing basis.

The risk assessment process is comprised of the following steps:
1. Prepare for the assessment
2. Conduct the assessment
3. Communicate the assessment
4. Maintain the assessment

## Step 1: Prepare for the Assessment

In this step, the objective is to establish context for the risk assessment. This can be accomplished by performing the following:

### Identify the purpose of the assessment
Determine the information that the assessment is intended to produce and the decisions the assessment is intended to support.

### Identify the scope of the assessment
Determine the organizational function or process that is applicable, the associated time frame and any applicable architectural or technological considerations.

### Identify any assumptions or constraints associated with the assessment
Determine assumptions in key areas relevant to the risk assessment including:
- Organizational priorities
- Business objectives
- Resource availability
- Skills and expertise of risk assessment team

### Identify sources of information
- Architectural / technological diagrams and system configurations
- Legal and regulatory requirements
- Threat Sources
- Threat Events
- Vulnerabilities and influencing conditions
- Potential Impacts
- Existing Controls

## Step 2: Conduct the Assessment

In this step, the objective is to produce a list of information security related risks that can be prioritized by risk level and used to inform risk response decisions. This can be accomplished by performing the following:

### Identify Threat Sources
Determine and characterize threat sources relevant to and of concern to Probably, Inc., including but not limited to:
- Human (Intentional or Unintentional / Internal or External)
- Environmental
- Natural
- System or Equipment

Consider the following when identifying threat sources:
- Capability
- Motive / Intent
- Intentionally targeted people, processes, and/or technologies
- Unintentionally targeted people, processes, and/or technologies

### Identify Threat Events
Determine what threat events could be produced by the identified threat sources that have potential to impact Probably, Inc..

Consider the relevance of the events and the sources that could initiate the events.

### Identify Vulnerabilities
Determine the vulnerabilities associated to people, process and/or technologies that could be exploited by the identified threat sources and threat events.

Consider any influencing conditions that could affect and aid in successful exploitation.

### Determine Likelihood
Determine the likelihood that the identified threat sources would initiate the identified threat events and could successfully exploit any identified vulnerabilities.

Consider the following when determining the likelihood:
- Characteristics of the threat sources that could initiate the events.
  - Capability
  - Motive/Intent
  - Opportunity
- The vulnerabilities and/or influencing conditions identified
- Probably, Inc.'s exposure based on any safeguards/countermeasures planned or implemented to prevent or mitigate such events.

### Determine Impact
Determine the impact to Probably, Inc.'s business objectives, operations, assets, individuals, customers, and/or other organizations by considering the following:
- Business / Operational Impacts
- Financial Damage
- Reputation Damage
- Legal or Regulatory Issues

When determining impact, also take into consideration any safeguards/countermeasures planned or implemented by Probably, Inc. that would mitigate or lessen the impact.

### Determine Risk
Determine the overall information security related risks to Probably, Inc. by combining the following:
- The likelihood of the event occurring.
- The impact that would result from the event.

The risk to Probably, Inc. is proportional to the likelihood and impact of an event.
- **Higher Risk Event:** Is more likely to occur and the resulting impact will be greater.
- **Lower Risk Event:** Is less likely to occur and the resulting impact will be minimal if any.

## Step 3: Communicate and Share the Risk Assessment Results

In this step, the objective is to ensure that decision makers across the Probably, Inc. and executive leadership have the appropriate risk-related information needed to inform and guide risk decisions.

### Communicate the Results
- Communicate the risk assessment results to Probably, Inc. decision maker and executive leadership to help drive risk based decisions and obtain the necessary support for the risk response.
- Share the risk assessment and risk-related information with the appropriate personnel at Probably, Inc. to help support the risk response efforts.

## Step 4: Maintain the Assessment

In this step, the objective is to keep current, the specific knowledge related to the risks that Probably, Inc. incurs. The results of the assessments inform, and drive risk based decisions and guide ongoing risk responses efforts.

### Monitor Risk Factors
Conduct ongoing monitoring of the risk factors that contribute to changes in risk to Probably, Inc.'s business objectives, operations, assets, individuals, customers, and/or other organizations.

### Maintain and Update the Assessment
Update existing risk assessments using the results from ongoing monitoring of risk factors and by conducting additional assessments, at minimum annually.

---

# APPENDIX B - Risk assessment matrix and description key

Each risk will be assessed as to its Likelihood and Impact. Likelihood can range from 1 ("Very unlikely") to 5 ("Very likely"). Impact can range from 1 ("Very low impact") to 5 ("Very high impact").

| Impact / Likelihood | Very unlikely: 1 | Unlikely: 2 | Somewhat likely: 3 | Likely: 4 | Very likely: 5 |
|---|---|---|---|---|---|
| **Very high impact: 5** | 5 | 10 | 15 | 20 | 25 |
| **High impact: 4** | 4 | 8 | 12 | 16 | 20 |
| **Medium impact: 3** | 3 | 6 | 9 | 12 | 15 |
| **Low impact: 2** | 2 | 4 | 6 | 8 | 10 |
| **Very low impact: 1** | 1 | 2 | 3 | 4 | 5 |

## Risk Level Descriptions

| Risk level | Description |
|------------|-------------|
| Low (1 - 4) | A threat event could be expected to have a limited adverse effect on organizational operations, mission capabilities, assets, individuals, customers, or other organizations. |
| Med (5 - 14) | A threat event could be expected to have a serious adverse effect on organizational operations, mission capabilities, assets, individuals, customers, or other organizations. |
| High (15 - 25) | A threat event could be expected to have a severe adverse effect on organizational operations, mission capabilities, assets, individuals, customers, or other organizations. |

## Impact Descriptions

| Impact | Description | Score |
|--------|-------------|-------|
| Very low impact (1) | A threat event could be expected to have almost no adverse effect on organizational operations, mission capabilities, assets, individuals, customers, or other organizations. | 1 |
| Low impact (2) | A threat event could be expected to have a limited adverse effect, meaning: degradation of mission capability yet primary functions can still be performed; minor damage; minor financial loss; or range of effects is limited to some cyber resources but no critical resources. | 2 |
| Medium impact (3) | A threat event could be expected to have a serious adverse effect, meaning: significant degradation of mission capability yet primary functions can still be performed at a reduced capacity; minor damage; minor financial loss; or range of effects is significant to some cyber resources and some critical resources. | 3 |
| High impact (4) | A threat event could be expected to have a severe or catastrophic adverse effect, meaning: severe degradation or loss of mission capability and one or more primary functions cannot be performed; major damage; major financial loss; or range of effects is extensive to most cyber resources and most critical resources. | 4 |
| Very high impact (5) | A threat event could be expected to have multiple severe or catastrophic adverse effects on organizational operations, assets, individuals, or other organizations. Range of effects is sweeping, involving almost all cyber resources. | 5 |

## Likelihood Descriptions

| Likelihood | Description | Score |
|------------|-------------|-------|
| Very unlikely (1) | A threat event is so unlikely that it can be assumed that its occurrence may not be experienced. A threat source is not motivated or has no capability, or controls are in place to prevent or significantly impede the vulnerability from being exploited. | 1 |
| Unlikely (2) | A threat event is unlikely, but there is a slight possibility that its occurrence may be experienced. A threat source lacks sufficient motivation or capability, or controls are in place to prevent or impede the vulnerability from being exploited. | 2 |
| Somewhat likely (3) | A threat event is likely, and it can be assumed that its occurrence may be experienced. A threat source is motivated or poses the capability, but controls are in place that may significantly reduce or impede the successful exploitation of the vulnerability. | 3 |
| Likely (4) | A threat event is likely, and it can be assumed that its occurrence will be experienced. A threat source is highly motivated or poses sufficient capability and resources, but some controls are in place that may reduce or impede the successful exploitation of the vulnerability. | 4 |
| Very likely (5) | A threat event is highly likely, and it can be assumed that its occurrence will be experienced. A threat source is highly motivated or poses sufficient capability or resources, but no controls are in place or controls that are in place are ineffective and do not prevent or impede the successful exploitation of the vulnerability. | 5 |